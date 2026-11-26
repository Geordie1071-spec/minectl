package modrinth

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
)

const rangeBlockSize = 256 << 10

type rangeReaderAt struct {
	http   *http.Client
	url    string
	size   int64
	blocks map[int64][]byte
}

func newRangeReaderAt(c *http.Client, url string) (*rangeReaderAt, error) {
	resp, err := c.Head(url)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HEAD %s: %s", url, resp.Status)
	}
	size, err := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	if err != nil || size <= 0 {
		return nil, fmt.Errorf("modpack file has no size")
	}
	return &rangeReaderAt{http: c, url: url, size: size, blocks: map[int64][]byte{}}, nil
}

func (r *rangeReaderAt) ReadAt(p []byte, off int64) (int, error) {
	n := 0
	for n < len(p) {
		pos := off + int64(n)
		if pos >= r.size {
			return n, io.EOF
		}
		start := pos / rangeBlockSize * rangeBlockSize
		block, err := r.block(start)
		if err != nil {
			return n, err
		}
		n += copy(p[n:], block[pos-start:])
	}
	return n, nil
}

func (r *rangeReaderAt) block(start int64) ([]byte, error) {
	if b, ok := r.blocks[start]; ok {
		return b, nil
	}
	end := min(start+rangeBlockSize, r.size) - 1
	req, err := http.NewRequest(http.MethodGet, r.url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	resp, err := r.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent {
		return nil, fmt.Errorf("range request: %s", resp.Status)
	}
	b := make([]byte, end-start+1)
	if _, err := io.ReadFull(resp.Body, b); err != nil {
		return nil, err
	}
	r.blocks[start] = b
	return b, nil
}
