/*
 * Copyright (c) 2023 Andreas Signer <asigner@gmail.com>
 *
 * This program is free software: you can redistribute it and/or modify it
 * under the terms of the GNU General Public License as published by the
 * Free Software Foundation, either version 3 of the License, or (at your
 * option) any later version.
 *
 * This program is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranty of MERCHANTABILITY
 * or FITNESS FOR A PARTICULAR PURPOSE. See the GNU General Public License
 * for more details.
 *
 * You should have received a copy of the GNU General Public License along
 * with this program. If not, see <https://www.gnu.org/licenses/>.
 */

package main

import (
	"errors"
	"fmt"
	"os"
	"time"
)

const (
	blockSize          = 512
	maxFilenameLen     = 22
	fileDescSize       = 32
	dirEntriesPerBlock = blockSize / fileDescSize
)

// ---------------------------------
// fileDesc
// ---------------------------------

type fileDesc struct {
	name       [maxFilenameLen]byte
	time, date int16
	head       int16
	size       int32
}

func (fd *fileDesc) nameAsString() string {
	i := 0
	for i < maxFilenameLen && fd.name[i] != 0 {
		i++
	}
	return string(fd.name[:i])
}

func (fd *fileDesc) timestamp() time.Time {
	// Oberon date and time format, according to "The Oberon System: User Guide and Programmer's Manual" by Martin Reiser
	// Date: 7 bits year, 4 bits month, 5 bits day
	// Time: 5 bits hour, 6 bits minute, 6 bits seconds.
	//
	// On floppy disks, the lowest bit of seconds is dropped

	y := 1900 + int(fd.date>>9&0x7f)
	m := time.Month(fd.date >> 5 & 0xf)
	d := int(fd.date & 0x1f)

	hh := int(fd.time >> 11 & 0x1f)
	mm := int(fd.time >> 5 & 0x6f)
	ss := int(fd.time&0x1f) * 2

	loc, _ := time.LoadLocation("Local")
	return time.Date(y, m, d, hh, mm, ss, 0, loc)
}

func fileDescFromBytes(buf []byte, ofs int) fileDesc {
	base := ofs * fileDescSize
	var fd fileDesc
	copy(fd.name[:], buf[base:base+maxFilenameLen])
	fd.time = int16(buf[base+23])<<8 | int16(buf[base+22])
	fd.date = int16(buf[base+25])<<8 | int16(buf[base+24])
	fd.head = int16(buf[base+27])<<8 | int16(buf[base+26])
	fd.size = int32(buf[base+31])<<24 | int32(buf[base+30])<<16 | int32(buf[base+29])<<8 | int32(buf[base+28])

	return fd
}

// ---------------------------------
// floppy
// ---------------------------------

type floppy struct {
	img []byte
	fat [720]int32
}

func (fl *floppy) getBlocks(idx, cnt int32) []byte {
	ofs := idx * blockSize
	return fl.img[ofs : ofs+cnt*blockSize]
}

func (fl *floppy) getBlock(idx int32) []byte {
	return fl.getBlocks(idx, 1)
}

func (fl *floppy) readDirBlock(block int32) []fileDesc {
	buf := fl.getBlock(block)
	var res []fileDesc
	for i := 0; i < dirEntriesPerBlock; i++ {
		res = append(res, fileDescFromBytes(buf, i))
	}
	return res
}

func (fl *floppy) initFAT() {
	buf := fl.getBlocks(1, 3)
	fl.fat[0] = -1
	fl.fat[1] = -1

	i := 2
	j := 3
	for i < 720 {
		n := int32(buf[j+2])<<16 | int32(buf[j+1])<<8 | int32(buf[j])
		n0 := n % 4096
		if n0 > 2047 {
			n0 -= 4096
		}
		n1 := n / 4096
		if n1 > 2047 {
			n1 -= 4096
		}
		fl.fat[i] = n0
		fl.fat[i+1] = n1
		i += 2
		j += 3
	}
}

func (fl *floppy) listFiles() ([]fileDesc, error) {
	// read boot sector
	buf := fl.getBlock(0)
	if buf[21] != 0xf9 && buf[21] != 0xe9 {
		return nil, errors.New("Neither Oberon nor MSDOS formatted diskette")
	}

	// Read volume label
	dbuf := fl.readDirBlock(7)
	fd := dbuf[0]
	if fd.name[11] != 8 {
		return nil, errors.New("Block 7 does not contain a valid volume label")
	}
	if fd.name[0] < 0xe5 && fd.name[0] != 0 {
		return nil, errors.New("Not Oberon format")
	}

	var res []fileDesc

	// read directory
	s := int32(7) // cur block
	j := 1        // index var in current block
	for {
		if dbuf[j].name[0] == 0 || dbuf[j].name[0] == 0xe5 {
			break
		}

		res = append(res, dbuf[j])

		j++
		if j == dirEntriesPerBlock {
			s++
			j = 0
			if s == 14 {
				break
			}
			dbuf = fl.readDirBlock(s)
		}
	}

	return res, nil
}

func (fl *floppy) readFile(fd fileDesc) ([]byte, error) {
	var res []byte
	var buf []byte

	remaining := fd.size
	if remaining == 0 {
		return res, nil
	}

	i := int32(fd.head)
	buf = fl.getBlocks(10+2*i, 2)
	for remaining > 1024 {
		res = append(res, buf...)
		remaining -= 1024
		i = fl.fat[i]
		buf = fl.getBlocks(10+2*i, 2)
	}
	res = append(res, buf[0:remaining]...)

	return res, nil
}

func newFloppy(filename string) *floppy {
	img, err := os.ReadFile(filename)
	if err != nil {
		panic(err)
	}
	fl := &floppy{img: img}
	fl.initFAT()
	return fl
}

// ------------------------------------------

type command func() error

func parseCommandLine(args []string) (cmd command, err error) {
	if len(args) == 0 {
		return printUsage, nil
	}
	imageFile := args[0]
	floppy := newFloppy(imageFile)
	i := 1
	switch args[i] {
	case "l", "list":
		// List command
		i++
		if i < len(args) {
			return nil, errors.New("unexpected args")
		}
		command := func() error {
			fds, err := floppy.listFiles()
			if err != nil {
				return err
			}
			for _, fd := range fds {
				fmt.Printf("%5d  %s  %-23s\n", fd.size, fd.timestamp().Format(time.DateTime), fd.nameAsString())
			}
			return nil
		}
		return command, nil
	case "d", "dump":
		// dump command
		i++
		if i >= len(args) {
			return nil, errors.New("filename missing")
		}
		toExtract := args[i]
		i++
		if i < len(args) {
			return nil, errors.New("unexpected args")
		}
		command := func() error {
			fds, err := floppy.listFiles()
			if err != nil {
				return err
			}
			for _, fd := range fds {
				if fd.nameAsString() != toExtract {
					continue
				}
				data, err := floppy.readFile(fd)
				if err != nil {
					return err
				}
				os.Stdout.Write(data)
				return nil
			}
			return fmt.Errorf("File %q not found", toExtract)
		}
		return command, nil
	case "x", "extract":
		// extract command
		i++
		if i >= len(args) {
			return nil, errors.New("filename missing")
		}
		toExtract := args[i]
		i++
		if i < len(args) {
			return nil, errors.New("unexpected args")
		}
		command := func() error {
			fds, err := floppy.listFiles()
			if err != nil {
				return err
			}
			for _, fd := range fds {
				if fd.nameAsString() != toExtract {
					continue
				}
				extractFile(floppy, fd)
				return nil
			}
			return fmt.Errorf("File %q not found", toExtract)
		}
		return command, nil
	case "xa", "extractall":
		i++
		if i < len(args) {
			return nil, errors.New("unexpected args")
		}
		command := func() error {
			fds, err := floppy.listFiles()
			if err != nil {
				return err
			}
			for _, fd := range fds {
				extractFile(floppy, fd)
			}
			return nil
		}
		return command, nil
	default:
		return nil, errors.New("unknown command")
	}
}

func extractFile(fl *floppy, fd fileDesc) error {
	data, err := fl.readFile(fd)
	if err != nil {
		return err
	}
	destName := fd.nameAsString()
	err = os.WriteFile(destName, data, 0666)
	if err != nil {
		return err
	}

	ts := fd.timestamp()
	err = os.Chtimes(destName, ts, ts)
	return err
}

func printUsage() error {
	fmt.Printf("Usage: cft <image file> command [command params]\n")
	fmt.Printf("Available commands are: (short form in parentheses)\n")
	fmt.Printf("  list (l): List all files\n")
	fmt.Printf("  dump (d) <filename>: Read file <filename> and write it to stdout\n")
	fmt.Printf("  extract (x) <filename>: Copy file <filename> to the current directory\n")
	fmt.Printf("  extractall (xa): Copy all files to the current directory\n")
	return nil
}

func main() {
	cmd, err := parseCommandLine(os.Args[1:])
	if err != nil {
		fmt.Printf("%s\n", err)
		printUsage()
		return
	}
	err = cmd()
	if err != nil {
		fmt.Printf("Error while executing command: %s\n", err)
	}
}
