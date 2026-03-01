package main

import (
	"bufio"
	"debug/elf"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const version = "0.1.2"

type multiFlag []string

type fwSection struct {
	Addr uint32
	Data []byte
}

type fwType int

const (
	fwRAW fwType = iota
	fwCXC
	fwBABE
)

type fwInfo struct {
	Path string
	Base uint32
	Type fwType
	Data []fwSection
}

const (
	DB2000  = 0x00010000
	DB2001  = 0x00030000
	DB2010  = 0x00100000
	DB2012  = 0x00300000
	PNX5230 = 0x01000000
	DB2020  = 0x10000000
)

const (
	COLOR_RED   = 0x00
	COLOR_BROWN = 0x60
	COLOR_BLUE  = 0xFFFFFFEF
)

type BABEHdr struct {
	Sig           uint16
	Unk           uint8
	Ver           int8
	Color         uint32
	Platform      uint32
	Z1            uint32
	CID           uint32
	Clr           uint32
	F0            [9]uint32
	CertPlace     [488]byte
	PrologueStart uint32
	PrologueSize1 uint32
	PrologueSize2 uint32
	Unk1          [4]uint32
	Hash1         [128]byte
	Flags         uint32
	Unk2          [4]uint32
	Clr2          uint32
	F1            [3]uint32
	PayloadStart  uint32
	PayloadSize1  uint32 // blocks
	PayloadSize2  uint32
	Flags2        uint32
	Unk4          [3]uint32
	Hash2         [128]byte
}

type CXCHdr struct {
	Unk             [0x20]byte
	Ver1            uint16
	Ver2            uint16
	HashHashtable   [0x14]byte
	HashTableOffset uint32
	HashTableLen    uint32
	HashBaseAddr    uint32
	CXCBodyOffset   uint32
	CXCLen          uint32
	BaseAddr        uint32
	HashCXC         [0x14]byte
	CXCLenForHash   uint32
}

const maxBlockSize = 64 * 1024 * 1024 // 64MB safety

type options struct {
	HasFirmware  bool
	BaseAddr     uint32
	ShowSections bool
	ChunkSize    int
}

func (m *multiFlag) String() string {
	return strings.Join(*m, ", ")
}

func (m *multiFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

func usage() {
	fmt.Println(`Usage:
elf2vkp-go -i <patch.elf> [options]

Options:
  -i <file>                 Input ELF patch (required)
  -f <file>                 Firmware file (BABE/CXC/RAW), can be repeated for multiple firmwares
  -b <addr>                 Firmware base address (default: 0)
  -o <file>                 Output VKP file (default: stdout). 
                             If multiple firmwares, output will be split automatically per firmware.

  --header <text>           Add header line (can be repeated)
  --header-from-file <file> Read header lines from a file

  --section-names           Show ELF section names in output
  --chunk-size <n>          Number of bytes per line (default: 16)

  -v, --version             Show version
  -h, --help                Show this help message`)
}

func isBABE(f *os.File) bool {
	var sig uint16
	err := binary.Read(io.NewSectionReader(f, 0, 2), binary.LittleEndian, &sig)
	if err != nil {
		return false
	}
	return sig == 0xBEBA
}

func parseBABE(f *os.File) ([]fwSection, error) {
	_, err := f.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}

	var hdr BABEHdr
	if err := binary.Read(f, binary.LittleEndian, &hdr); err != nil {
		return nil, err
	}

	// Signature
	if hdr.Sig != 0xBEBA {
		return nil, errors.New("not BABE (bad signature)")
	}

	// Version
	if hdr.Ver < 2 || hdr.Ver > 4 {
		return nil, errors.New("unsupported BABE version")
	}

	// Platform check
	switch hdr.Platform {
	case DB2000, DB2001, DB2010, DB2012, PNX5230, DB2020:
	default:
		return nil, errors.New("unsupported platform")
	}

	blocks := int(hdr.PayloadSize1)

	// Determine hash table size
	var hashTableSize uint32
	switch hdr.Ver {
	case 2:
		hashTableSize = 0x100
	case 3:
		hashTableSize = 1 * uint32(blocks)
	case 4:
		hashTableSize = 20 * uint32(blocks)
	}

	// Skip hash table
	if _, err := f.Seek(int64(hashTableSize), io.SeekCurrent); err != nil {
		return nil, err
	}

	var sections []fwSection

	// Read block table
	for range blocks {
		var addr uint32
		var size uint32

		if err := binary.Read(f, binary.LittleEndian, &addr); err != nil {
			return nil, err
		}
		if err := binary.Read(f, binary.LittleEndian, &size); err != nil {
			return nil, err
		}

		offset, err := f.Seek(0, io.SeekCurrent)
		if err != nil {
			return nil, err
		}

		if size == 0 || size > maxBlockSize {
			return nil, errors.New("invalid block size")
		}

		data := make([]byte, size)
		if _, err := f.Read(data); err != nil {
			return nil, err
		}

		sections = append(sections, fwSection{
			Addr: addr,
			Data: data,
		})

		if _, err := f.Seek(offset+int64(size), io.SeekStart); err != nil {
			return nil, err
		}
	}

	return sections, nil
}

func isCXC(f *os.File) bool {
	header := make([]byte, 0x24)
	_, err := f.ReadAt(header, 0)
	if err != nil {
		return false
	}

	return binary.LittleEndian.Uint16(header[0x20:]) == 2 &&
		binary.LittleEndian.Uint16(header[0x22:]) == 1
}

func parseCXC(f *os.File) ([]fwSection, error) {
	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}

	var hdr CXCHdr

	err = binary.Read(f, binary.LittleEndian, &hdr)
	if err != nil {
		return nil, err
	}

	if hdr.Ver1 != 2 || hdr.Ver2 != 1 {
		return nil, errors.New("not valid CXC (version mismatch)")
	}

	if (hdr.BaseAddr & 0x000FFFFF) != 0 {
		return nil, errors.New("invalid CXC base address alignment")
	}

	if hdr.HashTableOffset+hdr.HashTableLen != uint32(stat.Size()) {
		return nil, errors.New("invalid CXC hash table layout")
	}

	if hdr.CXCLen > maxBlockSize {
		return nil, errors.New("suspicious CXC size")
	}

	data := make([]byte, hdr.CXCLen)
	_, err = f.ReadAt(data, int64(hdr.CXCBodyOffset))
	if err != nil {
		return nil, err
	}

	return []fwSection{
		{
			Addr: hdr.BaseAddr,
			Data: data,
		},
	}, nil
}

func loadFirmware(f *os.File, rawBase uint32) ([]fwSection, fwType, error) {

	if isCXC(f) {
		sections, _ := parseCXC(f)
		return sections, fwCXC, nil
	}

	if isBABE(f) {
		sections, _ := parseBABE(f)
		return sections, fwBABE, nil
	}

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fwRAW, err
	}

	return []fwSection{{Addr: rawBase, Data: data}}, fwRAW, nil
}

func emitVKP(
	w io.Writer,
	addr uint32,
	oldData []byte,
	newData []byte,
	opts options,
	sectionName string,
) {
	if opts.ShowSections {
		fmt.Fprintf(w, "; section %s\n", sectionName)
	}

	for i := 0; i < len(newData); i += opts.ChunkSize {
		end := min(i+opts.ChunkSize, len(newData))

		newBytes := newData[i:end]
		offset := addr + uint32(i)

		if opts.HasFirmware {
			oldBytes := oldData[i:end]
			fmt.Fprintf(
				w,
				"%08X: %s %s\n",
				offset,
				strings.ToUpper(hex.EncodeToString(oldBytes)),
				strings.ToUpper(hex.EncodeToString(newBytes)),
			)
		} else {
			fmt.Fprintf(
				w,
				"%08X: %s\n",
				offset,
				strings.ToUpper(hex.EncodeToString(newBytes)),
			)
		}
	}
}

func main() {
	var (
		input        string
		firmwares    multiFlag
		baseStr      string
		output       string
		headerFile   string
		showHelp     bool
		showVersion  bool
		showSections bool
		chunkSize    int
		headers      multiFlag
	)

	flag.StringVar(&input, "i", "", "input ELF")
	flag.Var(&firmwares, "f", "firmware file (BABE/CXC/RAW)")
	flag.StringVar(&baseStr, "b", "0", "firmware base address")
	flag.StringVar(&output, "o", "", "output VKP")
	flag.Var(&headers, "header", "")
	flag.StringVar(&headerFile, "header-from-file", "", "")
	flag.BoolVar(&showSections, "section-names", false, "")
	flag.IntVar(&chunkSize, "chunk-size", 16, "")
	flag.BoolVar(&showHelp, "h", false, "")
	flag.BoolVar(&showHelp, "help", false, "")
	flag.BoolVar(&showVersion, "v", false, "")
	flag.BoolVar(&showVersion, "version", false, "")

	flag.Parse()

	if showHelp {
		usage()
		return
	}

	if showVersion {
		fmt.Println("elf2vkp-go", version)
		return
	}

	if input == "" {
		usage()
		os.Exit(1)
	}

	if chunkSize <= 0 {
		fmt.Fprintln(os.Stderr, "chunk-size must be > 0")
		os.Exit(1)
	}

	baseAddr64, err := strconv.ParseUint(strings.TrimPrefix(baseStr, "0x"), 16, 32)
	if err != nil {
		fmt.Fprintln(os.Stderr, "invalid base address")
		os.Exit(1)
	}
	baseAddr := uint32(baseAddr64)

	// -------------------------
	// Load all firmware first
	// -------------------------
	var fwList []fwInfo
	for _, fwPath := range firmwares {
		f, err := os.Open(fwPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to open firmware: %s\n", fwPath)
			os.Exit(1)
		}
		sections, ftype, err := loadFirmware(f, baseAddr)
		f.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to parse firmware %s: %v\n", fwPath, err)
			os.Exit(1)
		}
		if len(sections) == 0 {
			continue
		}
		fwList = append(fwList, fwInfo{
			Path: fwPath,
			Base: sections[0].Addr,
			Type: ftype,
			Data: sections,
		})
	}

	if len(fwList) == 0 {
		fwList = append(fwList, fwInfo{
			Path: "",
			Base: 0,
			Data: nil,
		})
	}

	// Sort by base address
	sort.Slice(fwList, func(i, j int) bool {
		return fwList[i].Base < fwList[j].Base
	})

	// Open ELF
	ef, err := elf.Open(input)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to open ELF")
		os.Exit(1)
	}
	defer ef.Close()

	// -------------------------
	// Process each firmware
	// -------------------------
	for i, fw := range fwList {

		lower := fw.Base
		upper := uint32(0xFFFFFFFF)
		if i+1 < len(fwList) {
			upper = fwList[i+1].Base
		}

		// ---------- OUTPUT ----------
		var out io.Writer = os.Stdout
		var outFile *os.File

		if output != "" {
			final := output
			if len(fwList) > 1 && fw.Path != "" {
				ext := filepath.Ext(output)
				base := strings.TrimSuffix(output, ext)
				name := strings.TrimSuffix(filepath.Base(fw.Path), filepath.Ext(fw.Path))
				final = fmt.Sprintf("%s_%s%s", base, name, ext)
			}
			outFile, err = os.Create(final)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to create %s\n", final)
				os.Exit(1)
			}
			out = outFile
		}

		// ---------- HEADER ----------
		for _, h := range headers {
			if !strings.HasPrefix(h, ";") {
				h = ";" + h
			}
			fmt.Fprintln(out, h)
		}

		if headerFile != "" {
			fh, err := os.Open(headerFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to open header file: %s\n", headerFile)
				os.Exit(1)
			}
			sc := bufio.NewScanner(fh)
			for sc.Scan() {
				fmt.Fprintln(out, sc.Text())
			}
			fh.Close()
		}

		// ---------- CXC PATCH FILE HEADER ----------
		if fw.Type == fwCXC && fw.Path != "" {
			name := strings.TrimSuffix(filepath.Base(fw.Path), filepath.Ext(fw.Path))
			fmt.Fprintf(out, ";pAtChFiLe=/boot/%s.cxc\n", name)
		}

		opts := options{
			HasFirmware:  fw.Path != "",
			BaseAddr:     lower,
			ShowSections: showSections,
			ChunkSize:    chunkSize,
		}

		// ---------- ELF Sections ----------
		for _, sec := range ef.Sections {

			if sec.Type != elf.SHT_PROGBITS || sec.Size == 0 || sec.Flags&elf.SHF_ALLOC == 0 {
				continue
			}

			addr := uint32(sec.Addr)

			if addr < lower || addr >= upper {
				continue
			}

			data, err := sec.Data()
			if err != nil || len(data) == 0 {
				continue
			}

			old := make([]byte, len(data))

			// Fill with 0xFF if BABE, otherwise 0x00
			if fw.Type == fwBABE {
				for i := range old {
					old[i] = 0xFF
				}
			}

			// Copy overlapping firmware bytes normally
			for _, s := range fw.Data {
				if addr >= s.Addr && addr < s.Addr+uint32(len(s.Data)) {
					offset := addr - s.Addr
					available := uint32(len(s.Data)) - offset
					toCopy := uint32(len(data))
					if toCopy > available {
						toCopy = available
					}
					copy(old[:toCopy], s.Data[offset:offset+toCopy])
				}
			}

			emitVKP(out, addr, old, data, opts, sec.Name)
		}

		// ---------- FOOTER ----------
		fmt.Fprintf(out, "; Generated by elf2vkp-go v%s\n", version)
		fmt.Fprintln(out, "; https://github.com/farid1991/elf2vkp-go")

		if outFile != nil {
			outFile.Close()
		}
	}
}
