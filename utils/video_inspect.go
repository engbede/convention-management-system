package utils

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// VideoMetadata contains parsed information about an inspected video file.
type VideoMetadata struct {
	Container     string
	VideoCodec    string
	AudioCodec    string
	HasVideoTrack bool
	HasAudioTrack bool
	IsWebPlayable bool
}

// ValidateVideoIntegrity inspects a video file or stream, verifying container structure,
// minimum size, and headers (such as MP4 ftyp and moov atoms).
// It runs efficiently using Seek, avoiding reading entire multi-gigabyte recordings into RAM.
func ValidateVideoIntegrity(r io.ReadSeeker, fileSize int64, filename string) (*VideoMetadata, error) {
	if r == nil {
		return nil, errors.New("no video data stream provided")
	}

	// 1. Determine size if not provided
	if fileSize <= 0 {
		cur, err := r.Seek(0, io.SeekEnd)
		if err != nil {
			return nil, fmt.Errorf("failed seeking video stream: %w", err)
		}
		fileSize = cur
	}

	// 2. Minimum size check: even minimal containers with moov/ftyp require at least 128 bytes
	const minValidVideoSize = 128
	if fileSize < minValidVideoSize {
		return nil, fmt.Errorf("video file is too small (%d bytes) to be a valid recording", fileSize)
	}

	ext := strings.ToLower(filepath.Ext(filename))

	// Reset stream to start
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed resetting stream to start: %w", err)
	}

	switch ext {
	case ".mp4", ".mov", ".m4v":
		return validateMP4Container(r, fileSize)
	case ".webm":
		return validateEBMLContainer(r, fileSize, "webm")
	case ".mkv":
		return validateEBMLContainer(r, fileSize, "matroska")
	case ".avi":
		return validateAVIContainer(r, fileSize)
	default:
		// Generic video check for unrecognized extensions
		return validateGenericVideo(r, fileSize)
	}
}

func validateMP4Container(r io.ReadSeeker, fileSize int64) (*VideoMetadata, error) {
	meta := &VideoMetadata{
		Container:     "mp4",
		IsWebPlayable: true,
	}

	var hasFTYP bool
	var hasMOOV bool
	var currentPos int64

	hdrBuf := make([]byte, 16)

	for currentPos+8 <= fileSize {
		if _, err := r.Seek(currentPos, io.SeekStart); err != nil {
			break
		}

		n, err := io.ReadFull(r, hdrBuf[:8])
		if err != nil || n < 8 {
			break
		}

		boxSize := int64(binary.BigEndian.Uint32(hdrBuf[0:4]))
		boxType := string(hdrBuf[4:8])
		headerSize := int64(8)

		if boxSize == 1 {
			// 64-bit extended box size
			nExt, errExt := io.ReadFull(r, hdrBuf[8:16])
			if errExt != nil || nExt < 8 {
				break
			}
			boxSize = int64(binary.BigEndian.Uint64(hdrBuf[8:16]))
			headerSize = 16
		} else if boxSize == 0 {
			// Box extends to end of file
			boxSize = fileSize - currentPos
		}

		if boxSize < headerSize {
			break
		}

		switch boxType {
		case "ftyp":
			hasFTYP = true
		case "moov":
			hasMOOV = true
			// Inspect moov contents to discover video/audio tracks
			inspectMoovAtom(r, currentPos+headerSize, boxSize-headerSize, meta)
		}

		currentPos += boxSize
	}

	if !hasFTYP {
		return nil, errors.New("invalid video container: missing MP4/QuickTime file type header (ftyp atom)")
	}

	if !hasMOOV {
		return nil, errors.New("video file is incomplete or corrupted: missing movie header (moov atom). The recording may have been interrupted or incompletely exported")
	}

	// Verify web playability based on detected codecs
	if meta.VideoCodec != "" {
		c := strings.ToLower(meta.VideoCodec)
		// avc1 = H.264, vp09 = VP9, av01 = AV1
		if c == "avc1" || c == "vp09" || c == "av01" {
			meta.IsWebPlayable = true
		} else {
			// e.g. hvc1/hev1 (HEVC) or prores may fail in some desktop browsers
			meta.IsWebPlayable = false
		}
	}

	return meta, nil
}

func inspectMoovAtom(r io.ReadSeeker, start int64, size int64, meta *VideoMetadata) {
	// Read up to 1 MB of moov atom metadata to discover tracks and codecs
	readSize := size
	if readSize > 1024*1024 {
		readSize = 1024 * 1024
	}

	if _, err := r.Seek(start, io.SeekStart); err != nil {
		return
	}

	data := make([]byte, readSize)
	n, err := io.ReadFull(r, data)
	if err != nil && err != io.ErrUnexpectedEOF {
		return
	}
	data = data[:n]

	// Search for track handlers and sample entries
	// Video handler pattern: "vide"
	// Audio handler pattern: "soun"
	if bytes.Contains(data, []byte("vide")) {
		meta.HasVideoTrack = true
	}
	if bytes.Contains(data, []byte("soun")) {
		meta.HasAudioTrack = true
	}

	// Search for video codec identifiers
	codecs := []string{"avc1", "hvc1", "hev1", "vp09", "av01", "mp4v"}
	for _, c := range codecs {
		if bytes.Contains(data, []byte(c)) {
			meta.VideoCodec = c
			break
		}
	}

	// Search for audio codec identifiers
	audioCodecs := []string{"mp4a", "opus", "ac-3", "alac"}
	for _, ac := range audioCodecs {
		if bytes.Contains(data, []byte(ac)) {
			meta.AudioCodec = ac
			break
		}
	}
}

func validateEBMLContainer(r io.ReadSeeker, fileSize int64, expectedDocType string) (*VideoMetadata, error) {
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	header := make([]byte, 512)
	n, err := io.ReadFull(r, header)
	if err != nil && err != io.ErrUnexpectedEOF {
		return nil, err
	}

	// EBML magic signature: 0x1A 0x45 0xDF 0xA3
	ebmlMagic := []byte{0x1A, 0x45, 0xDF, 0xA3}
	if !bytes.HasPrefix(header[:n], ebmlMagic) {
		return nil, errors.New("invalid WebM/MKV file: missing EBML container signature")
	}

	isWebPlayable := expectedDocType == "webm"

	return &VideoMetadata{
		Container:     expectedDocType,
		HasVideoTrack: true,
		IsWebPlayable: isWebPlayable,
	}, nil
}

func validateAVIContainer(r io.ReadSeeker, fileSize int64) (*VideoMetadata, error) {
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	header := make([]byte, 32)
	n, err := io.ReadFull(r, header)
	if err != nil && err != io.ErrUnexpectedEOF {
		return nil, err
	}

	// AVI starts with "RIFF" and has "AVI " at offset 8
	if n < 12 || string(header[0:4]) != "RIFF" || string(header[8:12]) != "AVI " {
		return nil, errors.New("invalid AVI file: missing RIFF/AVI container signature")
	}

	return &VideoMetadata{
		Container:     "avi",
		HasVideoTrack: true,
		IsWebPlayable: false, // AVI is not supported by HTML5 video tags in web browsers
	}, nil
}

func validateGenericVideo(r io.ReadSeeker, fileSize int64) (*VideoMetadata, error) {
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	buf := make([]byte, 512)
	n, _ := io.ReadFull(r, buf)

	return &VideoMetadata{
		Container:     "generic",
		HasVideoTrack: n > 0,
		IsWebPlayable: false,
	}, nil
}

// GenerateValidTestMP4 returns a minimal valid MP4 container with ftyp, moov (avc1 video track), and mdat atoms for testing.
func GenerateValidTestMP4() []byte {
	makeBox := func(name string, payload []byte) []byte {
		b := make([]byte, 8+len(payload))
		binary.BigEndian.PutUint32(b[0:4], uint32(len(b)))
		copy(b[4:8], name)
		copy(b[8:], payload)
		return b
	}

	ftyp := makeBox("ftyp", []byte("isom\x00\x00\x02\x00isomiso2"))
	hdlr := makeBox("hdlr", append([]byte("\x00\x00\x00\x00vide"), make([]byte, 12)...))
	stsd := makeBox("stsd", append([]byte("\x00\x00\x00\x00\x00\x00\x00\x01"), makeBox("avc1", append(make([]byte, 6), 0x00, 0x01))...))
	stbl := makeBox("stbl", stsd)
	minf := makeBox("minf", stbl)
	mdia := makeBox("mdia", append(hdlr, minf...))
	trak := makeBox("trak", mdia)
	mvhd := makeBox("mvhd", make([]byte, 100))
	moov := makeBox("moov", append(mvhd, trak...))
	mdat := makeBox("mdat", make([]byte, 64))

	result := make([]byte, 0, len(ftyp)+len(moov)+len(mdat))
	result = append(result, ftyp...)
	result = append(result, moov...)
	result = append(result, mdat...)
	return result
}
