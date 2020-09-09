package vegeta

import (
	"bufio"
	"errors"
	"io"
	"os"
	"sync"
)

var (
	instance     *Parameter
	once         sync.Once
	errs         error
	mu           sync.Mutex
	emptyDataErr = errors.New("data is empty")
)

type Parameter struct {
	filePath string
	recycle  bool
	reader   *bufio.Reader
	file     *os.File
	seek     int64
	first    bool
}

func NewParameter(recycle bool, filePath string) (*Parameter, error) {
	once.Do(func() {
		file, err := os.Open(filePath)
		if err != nil {
			instance, errs = nil, err
		}
		instance = &Parameter{
			filePath: filePath,
			recycle:  recycle,
			reader:   bufio.NewReader(file),
			file:     file,
			seek:     0,
			first:    true,
		}

	})

	return instance, errs
}

//Next get line data from *os.File
//return []byte and the data may Contains '\n' and Contains '\r'
func (p *Parameter) Next() (string, error) {
	mu.Lock()
	defer mu.Unlock()
	buf, err := p.reader.ReadSlice('\n')
	//buf := make([]byte, len(slice))
	//copy(buf, slice)
	//mu.Unlock()
	if err != nil {
		if err == io.EOF {
			if p.recycle {
				p.file.Seek(p.seek, io.SeekStart)
			} else {
				return string(buf), err
			}
		} else {
			return string(buf), err
		}

	}
	if p.first {
		p.seek = int64(len(buf))
		p.first = false
	}

	bytes, err := dropCRLF(buf)

	return string(bytes), err

}

//Close close the file
func (p *Parameter) Close() {
	mu.Lock()
	defer mu.Unlock()
	if p.file != nil {
		p.file.Close()
		p.file = nil
	}
}

//dropCRLF drop '\r' and '\n'
//if they exists in the []byte
func dropCRLF(data []byte) ([]byte, error) {
	var length = len(data)
	if length <= 2 {
		if length == 2 && data[length-2] == '\r' && data[length-1] == '\n' {
			return data[0:0], emptyDataErr
		}

		if length == 1 && data[length-1] == '\n' {
			return data[0:0], emptyDataErr
		}

		if length == 0 {
			return data[0:0], emptyDataErr
		}

	}

	if length > 2 && data[length-2] == '\r' {
		return data[0 : length-2], nil
	}
	if length > 1 && data[length-1] == '\n' {
		return data[0 : length-1], nil
	}

	return data[0:length], nil
}
