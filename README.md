# ELF2VKP-GO

Modern version of elf2vkp written in Go

This program:
1. Converts each .elf section into a V-Klay patch string.
2. Adds old patch data if a fullflash path is specified.
3. Supports ELF files produced by IAR and arm-none-eabi-gcc toolchains.
4. Supports BABE/CXC/RAW firmware formats.

# DOWNLOAD
- Windows: download .exe in [Releases](https://github.com/farid1991/elf2vkp-go/releases).
- Build from sources:
	```sh
 	go install github.com/farid1991/elf2vkp-go@latest
	```

# FEATURES
- Added support for BABE, CXC, and RAW firmware formats.
- Supports multiple firmware inputs and generates separate VKP outputs per firmware.
- Insert ";pAtChFiLe=/boot/<name>.cxc" header for CXC firmware automatically.
- Fill old bytes with 0xFF for BABE firmware, 0x00 otherwise.
- Handles overlapping firmware sections correctly in emitVKP.
- Preserves user-provided headers and headers from files.

# USAGE
```sh
$ elf2vkp-go -i <patch.elf> [options]

Options:
  -i <file>                 Input ELF patch (required)
  -f <file>                 Firmware file (BABE/CXC/RAW), 
                            can be repeated for multiple firmwares
  -b <addr>                 Firmware base address (default: 0)
  -o <file>                 Output VKP file (default: stdout)
                            If multiple firmwares are specified, 
							outputs will be split automatically per firmware file.

  --header <text>           Add header line (repeatable)
  --header-from-file <file> Read header lines from a file

  --section-names           Show ELF section names in output
  --chunk-size <n>          Number of bytes per line (default: 16)

  -v, --version             Show version
  -h, --help                Show this help message
```

## EXAMPLE

### Convert patch.elf to patch.vkp with old data (RAW Firmware)
```sh
$ elf2vkp-go -i patch.elf -o patch.vkp -f U10_R7AA071.bin -b 0x14000000
```

### Convert patch.elf to patch.vkp with old data (CXC Firmware)
```sh
$ elf2vkp-go -i patch.elf -o patch.vkp -f U10_R7AA071/phone_app.cxc
```

### Convert patch.elf to patch.vkp without old data
```sh
$ elf2vkp-go -i patch.elf -o patch.vkp
```

### Convert patch.elf to patch.vkp and print to stdout
```sh
$ elf2vkp-go -i patch.elf
```

### Convert patch.elf with multiple firmware files (automatic output split)
```sh
elf2vkp-go -i patch.elf -f phone_emp_app.cxc -f phone_app.cxc -o patch.vkp

# Outputs: patch_phone_emp.vkp and patch_phone_app.vkp
```