# ELF2VKP-GO

Modern version of elf2vkp written in Go

This program:
1. Converts each .elf section into a V-Klay patch string.
2. Adds old patch data if a fullflash path is specified.
3. Supports ELF files produced by IAR and arm-none-eabi-gcc toolchains.

# DOWNLOAD
- Windows: download .exe in [Releases](https://github.com/farid1991/elf2vkp-go/releases).
- Build from sources:
	```sh
 	go install github.com/farid1991/elf2vkp-go@latest
	```

# USAGE
```sh
Usage:
elf2vkp-go -i patch.elf [options]

Options:
-i <file>              Input ELF patch
-f <file>              Firmware BIN (optional)
-b <addr>              Firmware base address (default: 0)
-o <file>              Output VKP (default: stdout)

--header <text>        Add header line (repeatable)
--header-from-file <f> Read header lines from file

--section-names        Show ELF section names
--chunk-size <n>       Bytes per line (default: 16)

-v, --version          Show version
-h, --help             Show help
```

### Convert patch.elf to patch.vkp with old data
```
$ elf2vkp-go -i patch.elf -o patch.vkp -f U10_R7AA071.bin -b 0x14000000
```

### Convert patch.elf to patch.vkp without old data
```
$ elf2vkp-go -i patch.elf -o patch.vkp
```

### Convert patch.elf to patch.vkp and print to stdout
```
$ elf2vkp-go -i patch.elf
```