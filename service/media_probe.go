package service

import "bytes"

// classifyContainerMime refines a sniffed MIME for containers that can hold
// either audio or video. When a file has an audio track but no video track it
// returns the audio-only MIME (audio/webm, audio/mp4, audio/ogg) — all valid
// IANA types — so it is classified and filtered as audio. Otherwise the
// original type is returned unchanged.
func classifyContainerMime(mimeType string, data []byte) string {
	switch mimeType {
	case "video/webm":
		if !webmHasVideo(data) && webmHasAudio(data) {
			return "audio/webm"
		}
	case "video/mp4", "video/quicktime":
		if !mp4HasVideo(data) && mp4HasAudio(data) {
			return "audio/mp4"
		}
	case "video/ogg":
		if !oggHasVideo(data) && oggHasAudio(data) {
			return "audio/ogg"
		}
	case "application/ogg":
		switch {
		case oggHasVideo(data):
			return "video/ogg"
		case oggHasAudio(data):
			return "audio/ogg"
		}
	}
	return mimeType
}

// webmHasVideo/webmHasAudio look for the Matroska TrackType element (ID 0x83,
// 1-byte size 0x81, value 1=video / 2=audio).
func webmHasVideo(data []byte) bool {
	return bytes.Contains(data, []byte{0x83, 0x81, 0x01})
}

func webmHasAudio(data []byte) bool {
	return bytes.Contains(data, []byte{0x83, 0x81, 0x02})
}

func oggHasVideo(data []byte) bool {
	return bytes.Contains(data, []byte("theora"))
}

func oggHasAudio(data []byte) bool {
	return bytes.Contains(data, []byte("OpusHead")) ||
		bytes.Contains(data, []byte("\x01vorbis")) ||
		bytes.Contains(data, []byte("Speex"))
}

func mp4HasVideo(data []byte) bool { return mp4Handler(data, "vide") }
func mp4HasAudio(data []byte) bool { return mp4Handler(data, "soun") }

// mp4Handler scans ISO-BMFF boxes for an `hdlr` box whose handler type (four
// bytes 12 after the "hdlr" marker) matches the given handler.
func mp4Handler(data []byte, handler string) bool {
	marker := []byte("hdlr")
	want := []byte(handler)
	for offset := 0; ; {
		i := bytes.Index(data[offset:], marker)
		if i < 0 {
			return false
		}
		pos := offset + i
		handlerPos := pos + 4 + 4 + 4 // version+flags(4) + pre_defined(4)
		if handlerPos+4 <= len(data) && bytes.Equal(data[handlerPos:handlerPos+4], want) {
			return true
		}
		offset = pos + 4
	}
}
