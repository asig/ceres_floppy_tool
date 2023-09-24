# Ceres Floppy Tool

`ceres_floppy_tool` is a small program that allows you to extract files from
floppy images of floppies that you used with your old [Ceres](https://en.wikipedia.org/wiki/Ceres_(workstation))

## Building ceres_floppy_tool
The source code is in a single file, and does not require any non-standard 
go dependencies, so all you need to do is `go build ceres_floppy_tool.go`.

If you have the `make`` installed (and you proably do if you're reading this), then
you can also just call `make`.

## Usage

Usage: `ceres_floppy_tool <image-file> <command> [command params]`

`image-file` is a raw floppy dump that can be generated with a standard USB floppy drive and `dd`

Available commands:
   - `list` or `l`: Lists all the files that are stored in the floppy image.
   - `dump` or `d`: Dumps a file to stdout. The filename of the file to be dumped is the only parameter to this command.
   - `extract` or `x`: Copies a single file from the image to the current directory. The filename of the file to be extracte is the only parameter to this command.
   - `extractall` or `xa`: Copies all files available in the image to the current directory.

## License
Copyright (c) 2023 Andreas Signer.  
Licensed under [GPLv3](https://www.gnu.org/licenses/gpl-3.0).
