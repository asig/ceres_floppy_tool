.PHONY: clean

all: ceres_floppy_tool

ceres_floppy_tool: ceres_floppy_tool.go
	go build ceres_floppy_tool.go

clean:
	rm -f ceres_floppy_tool

