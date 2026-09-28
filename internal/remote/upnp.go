package remote

import (
	"bufio"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

// ssdpAddr ist eine Variable, damit Tests einen Fake-Router auf Loopback nutzen können.
var ssdpAddr = "239.255.255.250:1900"

// upnpMap sucht per SSDP ein Internet Gateway Device und gibt dort den Port frei.
func upnpMap(ctx context.Context, port int) (*mapping, error) {
	locs, err := ssdpSearch(ctx)
	if err != nil {
		return nil, err
	}
	var last error = errors.New("UPnP: kein Router antwortet")
	for _, loc := range locs {
		m, err := upnpMapAt(ctx, loc, port)
		if err == nil {
			return m, nil
		}
		last = err
	}
	return nil, last
}

func ssdpSearch(ctx context.Context) ([]string, error) {
	conn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	dst, err := net.ResolveUDPAddr("udp4", ssdpAddr)
	if err != nil {
		return nil, err
	}
	for _, st := range []string{"urn:schemas-upnp-org:device:InternetGatewayDevice:1", "urn:schemas-upnp-org:device:InternetGatewayDevice:2"} {
		msg := "M-SEARCH * HTTP/1.1\r\nHOST: 239.255.255.250:1900\r\nMAN: \"ssdp:discover\"\r\nMX: 2\r\nST: " + st + "\r\n\r\n"
		if _, err := conn.WriteTo([]byte(msg), dst); err != nil {
			return nil, err
		}
	}
	deadline := time.Now().Add(3 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	conn.SetReadDeadline(deadline)
	var locs []string
	buf := make([]byte, 2048)
	for {
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			break // Deadline
		}
		resp, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(buf[:n])), nil)
		if err != nil {
			continue
		}
		if loc := resp.Header.Get("Location"); loc != "" && !slices.Contains(locs, loc) {
			locs = append(locs, loc)
		}
	}
	if len(locs) == 0 {
		return nil, errors.New("UPnP: kein Router antwortet (UPnP im Router aus?)")
	}
	return locs, nil
}

type upnpDevice struct {
	Services []struct {
		Type       string `xml:"serviceType"`
		ControlURL string `xml:"controlURL"`
	} `xml:"serviceList>service"`
	Devices []upnpDevice `xml:"deviceList>device"`
}

func (d upnpDevice) wan() (typ, ctrl string) {
	for _, s := range d.Services {
		if strings.HasPrefix(s.Type, "urn:schemas-upnp-org:service:WANIPConnection:") ||
			strings.HasPrefix(s.Type, "urn:schemas-upnp-org:service:WANPPPConnection:") {
			return s.Type, s.ControlURL
		}
	}
	for _, c := range d.Devices {
		if t, u := c.wan(); t != "" {
			return t, u
		}
	}
	return "", ""
}

// upnpMapAt liest die Gerätebeschreibung unter loc und legt die Freigabe an.
func upnpMapAt(ctx context.Context, loc string, port int) (*mapping, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", loc, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var root struct {
		URLBase string     `xml:"URLBase"`
		Device  upnpDevice `xml:"device"`
	}
	if err := xml.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&root); err != nil {
		return nil, fmt.Errorf("UPnP-Beschreibung: %w", err)
	}
	svc, ctrl := root.Device.wan()
	if svc == "" {
		return nil, errors.New("UPnP: Router bietet keine Portfreigabe an")
	}
	base, _ := url.Parse(loc)
	if root.URLBase != "" {
		if b, err := url.Parse(root.URLBase); err == nil {
			base = b
		}
	}
	c, err := base.Parse(ctrl)
	if err != nil {
		return nil, err
	}
	ctrlURL := c.String()

	local, err := localIPTo(base.Hostname())
	if err != nil {
		return nil, err
	}
	out, err := soap(ctx, ctrlURL, svc, "GetExternalIPAddress", nil)
	if err != nil {
		return nil, err
	}
	m := &mapping{method: "upnp", extIP: net.ParseIP(xmlValue(out, "NewExternalIPAddress")), extPort: port, lease: time.Hour}
	add := func(ctx context.Context) error {
		args := func(lease time.Duration) [][2]string {
			return [][2]string{{"NewRemoteHost", ""}, {"NewExternalPort", strconv.Itoa(port)}, {"NewProtocol", "TCP"},
				{"NewInternalPort", strconv.Itoa(port)}, {"NewInternalClient", local.String()}, {"NewEnabled", "1"},
				{"NewPortMappingDescription", "Flimmer"}, {"NewLeaseDuration", strconv.Itoa(int(lease.Seconds()))}}
		}
		_, err := soap(ctx, ctrlURL, svc, "AddPortMapping", args(m.lease))
		if err != nil && strings.Contains(err.Error(), "725") { // OnlyPermanentLeasesSupported
			m.lease = 0
			_, err = soap(ctx, ctrlURL, svc, "AddPortMapping", args(0))
		}
		return err
	}
	if err := add(ctx); err != nil {
		return nil, err
	}
	m.renew = add
	m.remove = func(ctx context.Context) error {
		_, err := soap(ctx, ctrlURL, svc, "DeletePortMapping",
			[][2]string{{"NewRemoteHost", ""}, {"NewExternalPort", strconv.Itoa(port)}, {"NewProtocol", "TCP"}})
		return err
	}
	return m, nil
}

func soap(ctx context.Context, ctrlURL, svc, action string, args [][2]string) ([]byte, error) {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/"><s:Body><u:`)
	b.WriteString(action + ` xmlns:u="` + svc + `">`)
	for _, a := range args {
		b.WriteString("<" + a[0] + ">")
		xml.EscapeText(&b, []byte(a[1]))
		b.WriteString("</" + a[0] + ">")
	}
	b.WriteString("</u:" + action + "></s:Body></s:Envelope>")
	req, err := http.NewRequestWithContext(ctx, "POST", ctrlURL, strings.NewReader(b.String()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", `text/xml; charset="utf-8"`)
	req.Header.Set("SOAPAction", `"`+svc+"#"+action+`"`)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("UPnP %s: Fehler %s %s", action, xmlValue(body, "errorCode"), xmlValue(body, "errorDescription"))
	}
	return body, nil
}

// xmlValue liefert den Text des ersten Elements mit diesem lokalen Namen.
func xmlValue(body []byte, name string) string {
	d := xml.NewDecoder(bytes.NewReader(body))
	for {
		t, err := d.Token()
		if err != nil {
			return ""
		}
		if se, ok := t.(xml.StartElement); ok && se.Name.Local == name {
			var v string
			d.DecodeElement(&v, &se)
			return strings.TrimSpace(v)
		}
	}
}
