package ffmpeg

import "testing"

func TestPickAsset(t *testing.T) {
	sums := []byte(`7be4e989f0b038d3cc777d97469e2c1b0db07b6eacb64b04d6db7509c1fc3fe7  ffmpeg-master-latest-win64-gpl.zip
36189b91862a6bf75c57223887c36c73654e3ada01d8bea64d5e1ba667ee7b7b  ffmpeg-n8.1-latest-win64-gpl-8.1.zip
e6db684f1527f4c2280b017c7af19ebd359424eee8b35974bc35b4d7ee110989  ffmpeg-n9.0-latest-win64-gpl-9.0.zip
14a3dc0a93aac34e531b69f337d6f0dcb525491918c0b19b2bce936dcbc9822b  ffmpeg-n9.0-latest-win64-gpl-shared-9.0.zip
`)
	name, sum, err := pickAsset(sums)
	if err != nil || name != "ffmpeg-n9.0-latest-win64-gpl-9.0.zip" || sum[:8] != "e6db684f" {
		t.Fatalf("%s %s %v", name, sum, err)
	}
}
