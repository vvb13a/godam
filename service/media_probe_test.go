package service

import "testing"

func mp4WithHandler(handler string) []byte {
	b := []byte("....ftypisom....hdlr")
	b = append(b, make([]byte, 8)...) // version+flags + pre_defined
	b = append(b, []byte(handler)...) // handler_type
	return b
}

func TestClassifyContainerMime(t *testing.T) {
	webmAudio := []byte{0x1A, 0x45, 0xDF, 0xA3, 0x83, 0x81, 0x02}
	webmVideo := []byte{0x1A, 0x45, 0xDF, 0xA3, 0x83, 0x81, 0x01}
	oggAudio := []byte("OggS\x00\x02....OpusHead")
	oggVideo := []byte("OggS\x00\x02....\x80theora")

	cases := []struct {
		name string
		mime string
		data []byte
		want string
	}{
		{"webm audio-only", "video/webm", webmAudio, "audio/webm"},
		{"webm with video", "video/webm", webmVideo, "video/webm"},
		{"mp4 audio-only", "video/mp4", mp4WithHandler("soun"), "audio/mp4"},
		{"mp4 with video", "video/mp4", mp4WithHandler("vide"), "video/mp4"},
		{"quicktime audio-only", "video/quicktime", mp4WithHandler("soun"), "audio/mp4"},
		{"ogg generic audio", "application/ogg", oggAudio, "audio/ogg"},
		{"ogg generic video", "application/ogg", oggVideo, "video/ogg"},
		{"ogg video stays video", "video/ogg", oggVideo, "video/ogg"},
		{"plain mime untouched", "image/png", []byte("x"), "image/png"},
	}
	for _, c := range cases {
		if got := classifyContainerMime(c.mime, c.data); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
