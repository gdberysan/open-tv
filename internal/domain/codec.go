package domain

import (
	"fmt"
	"strings"
)

// CodecSupport dice si el VÍDEO de un stream lo decodifica un navegador.
//
// Hermano de WebSupport y AirplaySupport en forma —tri-estado, "no se sabe"
// es un veredicto legítimo— pero con OTRO significado: WebNo dice «no
// directo, ve por el proxy»; CodecNo dice «ningún navegador lo decodifica,
// ni lo intentes». Caso medido (2026-09-05): AMC (720p) emite vídeo MPEG-2;
// hls.js lo tira en silencio y Safari da audio sin imagen.
type CodecSupport int

const (
	CodecUnknown CodecSupport = iota
	CodecNo
	CodecOK
)

// Tipos de stream elemental de MPEG-TS (ISO/IEC 13818-1, tabla 2-34, más
// los registrados por ATSC que ffmpeg y hls.js reconocen).
const (
	tsVideoMPEG2 byte = 0x02
	tsAudioMPEG1 byte = 0x03
	tsAudioMPEG2 byte = 0x04
	tsAudioAAC   byte = 0x0f
	tsVideoMPEG4 byte = 0x10
	tsAudioLATM  byte = 0x11
	tsVideoH264  byte = 0x1b
	tsVideoHEVC  byte = 0x24
	tsAudioAC3   byte = 0x81
	tsAudioEAC3  byte = 0x87
	tsVideoVC1   byte = 0xea
)

// AudioSupport dice si la PMT trae alguna pista de audio. AudioNo no es un
// defecto de códec: es un origen que emite vídeo mudo (AMC mirror 2). El
// cliente lo relega y lo etiqueta («Sin audio en este origen»), no lo salta.
type AudioSupport int

const (
	AudioUnknown AudioSupport = iota
	AudioNo
	AudioOK
)

// ClassifyAudio: basta un stream de audio conocido. Sin streams no se juzga.
func ClassifyAudio(streams []StreamTS) AudioSupport {
	if len(streams) == 0 {
		return AudioUnknown
	}
	for _, s := range streams {
		switch s.Tipo {
		case tsAudioMPEG1, tsAudioMPEG2, tsAudioAAC, tsAudioLATM, tsAudioAC3, tsAudioEAC3:
			return AudioOK
		}
	}
	return AudioNo
}

// ClassifyCodecs aplica una regla centrada en el vídeo: basta un stream
// H.264 para que sirva; si hay vídeo y ninguno es H.264, no sirve; sin
// vídeo no se juzga (el solo-audio no es asunto de este veredicto). HEVC
// queda fuera igual que en codecsWeb: solo Safari lo abre.
func ClassifyCodecs(streams []StreamTS) CodecSupport {
	hayVideo := false
	for _, s := range streams {
		switch s.Tipo {
		case tsVideoH264:
			return CodecOK
		case tsVideoMPEG2, tsVideoMPEG4, tsVideoHEVC, tsVideoVC1:
			hayVideo = true
		}
	}
	if hayVideo {
		return CodecNo
	}
	return CodecUnknown
}

// NombreCodecs devuelve una cadena corta para stats y para el mensaje al
// usuario, con los nombres que usa ffprobe. Un tipo desconocido sale en
// hexadecimal para que el censo lo pueda contar.
func NombreCodecs(streams []StreamTS) string {
	nombres := make([]string, 0, len(streams))
	for _, s := range streams {
		nombres = append(nombres, nombreTipoTS(s.Tipo))
	}
	return strings.Join(nombres, ",")
}

// CodecsDeVideo filtra la lista completa de NombreCodecs (todo el PMT, en
// SU orden) y se queda solo con los nombres de VÍDEO conocidos, descartando
// audio y tipos desconocidos (los "0x.."). Es lo que viaja por el cable en
// /channels/streams: la columna codecs de la DB guarda el PMT entero para
// el censo, pero el mensaje al usuario («el vídeo viene en X») solo debe
// nombrar vídeo, nunca un códec de audio.
func CodecsDeVideo(cadena string) string {
	if cadena == "" {
		return ""
	}
	video := make([]string, 0, 2)
	for _, nombre := range strings.Split(cadena, ",") {
		switch nombre {
		case "mpeg2video", "mpeg4", "h264", "hevc", "vc1":
			video = append(video, nombre)
		}
	}
	return strings.Join(video, ",")
}

func nombreTipoTS(tipo byte) string {
	switch tipo {
	case tsVideoMPEG2:
		return "mpeg2video"
	case tsAudioMPEG1, tsAudioMPEG2:
		return "mp2"
	case tsAudioAAC, tsAudioLATM:
		return "aac"
	case tsVideoMPEG4:
		return "mpeg4"
	case tsVideoH264:
		return "h264"
	case tsVideoHEVC:
		return "hevc"
	case tsAudioAC3:
		return "ac3"
	case tsAudioEAC3:
		return "eac3"
	case tsVideoVC1:
		return "vc1"
	default:
		return fmt.Sprintf("0x%02x", tipo)
	}
}
