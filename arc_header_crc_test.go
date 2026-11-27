package rardecode

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"os"
	"testing"
)

// RAR 3.x technote (technote.txt, "Archive header ( MAIN_HEAD )"): HEAD_CRC is the "CRC of fields HEAD_TYPE to RESERVED2" (low 16 bits of CRC32), and flag
// 0x0080 means "Block headers are encrypted". The marker block is 7 bytes; the main header follows: HEAD_CRC (2), HEAD_TYPE (1), HEAD_FLAGS (2), HEAD_SIZE (2).
func mainHeaderWithFlags(t *testing.T, flags uint16, fixCRC bool) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/testfile.rar3.rar")
	if err != nil {
		t.Fatal(err)
	}
	b = bytes.Clone(b)
	binary.LittleEndian.PutUint16(b[10:], flags)
	if fixCRC {
		size := int(binary.LittleEndian.Uint16(b[12:]))
		binary.LittleEndian.PutUint16(b[7:], uint16(crc32.ChecksumIEEE(b[9:7+size])))
	}
	return b
}

// A main header whose flags were changed and whose CRC no longer holds is a corrupt header, not an archive that states encrypted headers.
func TestMainHeaderFlagsAreNotReadBeforeTheCRC(t *testing.T) {
	_, err := NewReader(bytes.NewReader(mainHeaderWithFlags(t, 0x00ff, false)))
	if !errors.Is(err, ErrBadHeaderCRC) {
		t.Errorf("flags 0x00ff with the old CRC: %v, want ErrBadHeaderCRC", err)
	}
	// the same flags with a CRC that holds: the header does state encrypted headers
	_, err = NewReader(bytes.NewReader(mainHeaderWithFlags(t, 0x0080, true)))
	if !errors.Is(err, ErrArchiveEncrypted) {
		t.Errorf("flags 0x0080 with a valid CRC: %v, want ErrArchiveEncrypted", err)
	}
	// unchanged file
	if _, err = NewReader(bytes.NewReader(mainHeaderWithFlags(t, 0, true))); err != nil {
		t.Errorf("flags 0 with a valid CRC: %v", err)
	}
}
