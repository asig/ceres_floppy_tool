.PHONY: clean

all: cft

cft: cft.go
	go build cft.go

clean:
	rm -f cft

