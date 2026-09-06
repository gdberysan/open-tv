package domain_test

import (
	"testing"

	"github.com/gdberysan/open-tv/internal/domain"
)

func TestClassifyCodecs(t *testing.T) {
	casos := []struct {
		nombre  string
		streams []domain.StreamTS
		quiero  domain.CodecSupport
	}{
		{"MPEG-2 + MP2 (AMC mirror 1)", []domain.StreamTS{{Tipo: 0x02}, {Tipo: 0x03}}, domain.CodecNo},
		{"H.264 + AAC", []domain.StreamTS{{Tipo: 0x1b}, {Tipo: 0x0f}}, domain.CodecOK},
		{"H.264 solo, sin audio (AMC mirror 2)", []domain.StreamTS{{Tipo: 0x1b}}, domain.CodecOK},
		{"H.264 + AC-3: la regla es de vídeo", []domain.StreamTS{{Tipo: 0x1b}, {Tipo: 0x81}}, domain.CodecOK},
		{"HEVC + AAC", []domain.StreamTS{{Tipo: 0x24}, {Tipo: 0x0f}}, domain.CodecNo},
		{"MPEG-4 parte 2", []domain.StreamTS{{Tipo: 0x10}}, domain.CodecNo},
		{"VC-1", []domain.StreamTS{{Tipo: 0xea}}, domain.CodecNo},
		{"solo audio: no se juzga", []domain.StreamTS{{Tipo: 0x0f}}, domain.CodecUnknown},
		{"vacío", nil, domain.CodecUnknown},
	}
	for _, c := range casos {
		if got := domain.ClassifyCodecs(c.streams); got != c.quiero {
			t.Errorf("%s: ClassifyCodecs = %v, quiero %v", c.nombre, got, c.quiero)
		}
	}
}

func TestNombreCodecs(t *testing.T) {
	casos := []struct {
		streams []domain.StreamTS
		quiero  string
	}{
		{[]domain.StreamTS{{Tipo: 0x02}, {Tipo: 0x03}}, "mpeg2video,mp2"},
		{[]domain.StreamTS{{Tipo: 0x1b}, {Tipo: 0x0f}}, "h264,aac"},
		{[]domain.StreamTS{{Tipo: 0x1b}}, "h264"},
		{[]domain.StreamTS{{Tipo: 0x24}, {Tipo: 0x81}}, "hevc,ac3"},
		{[]domain.StreamTS{{Tipo: 0x1b}, {Tipo: 0x99}}, "h264,0x99"},
		{nil, ""},
	}
	for _, c := range casos {
		if got := domain.NombreCodecs(c.streams); got != c.quiero {
			t.Errorf("NombreCodecs(%+v) = %q, quiero %q", c.streams, got, c.quiero)
		}
	}
}

func TestCodecsDeVideo(t *testing.T) {
	casos := []struct {
		cadena string
		quiero string
	}{
		{"aac,hevc", "hevc"},
		{"mp2,mpeg2video", "mpeg2video"},
		{"hevc,aac,0x86,0x15", "hevc"},
		{"h264,aac", "h264"},
		{"aac", ""},
		{"", ""},
	}
	for _, c := range casos {
		if got := domain.CodecsDeVideo(c.cadena); got != c.quiero {
			t.Errorf("CodecsDeVideo(%q) = %q, quiero %q", c.cadena, got, c.quiero)
		}
	}
}

func TestClassifyAudio(t *testing.T) {
	casos := []struct {
		nombre  string
		streams []domain.StreamTS
		quiero  domain.AudioSupport
	}{
		{"H.264 + AAC", []domain.StreamTS{{Tipo: 0x1b}, {Tipo: 0x0f}}, domain.AudioOK},
		{"MPEG-2 + MP2", []domain.StreamTS{{Tipo: 0x02}, {Tipo: 0x03}}, domain.AudioOK},
		{"H.264 + AC-3", []domain.StreamTS{{Tipo: 0x1b}, {Tipo: 0x81}}, domain.AudioOK},
		{"H.264 solo (AMC mirror 2)", []domain.StreamTS{{Tipo: 0x1b}}, domain.AudioNo},
		{"solo audio", []domain.StreamTS{{Tipo: 0x0f}}, domain.AudioOK},
		{"vacío", nil, domain.AudioUnknown},
	}
	for _, c := range casos {
		if got := domain.ClassifyAudio(c.streams); got != c.quiero {
			t.Errorf("%s: ClassifyAudio = %v, quiero %v", c.nombre, got, c.quiero)
		}
	}
}
