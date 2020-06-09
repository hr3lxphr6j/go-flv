//
// Copyright (c) 2018- yutopp (yutopp@gmail.com)
//
// Distributed under the Boost Software License, Version 1.0. (See accompanying
// file LICENSE_1_0.txt or copy at  https://www.boost.org/LICENSE_1_0.txt)
//

package flv

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/pkg/errors"

	"github.com/yutopp/go-flv/pool"
	"github.com/yutopp/go-flv/tag"
)

type Decoder struct {
	r           io.Reader
	header      *Header
	decodedOnce bool
}

func NewDecoder(r io.Reader) (*Decoder, error) {
	header, err := DecodeFlvHeader(r)
	if err != nil {
		return nil, err
	}

	if header.DataOffset > HeaderLength {
		offset := header.DataOffset - HeaderLength
		if _, err := io.CopyN(ioutil.Discard, r, int64(offset)); err != nil {
			return nil, err
		}
	}

	return &Decoder{
		r:      r,
		header: header,
	}, nil
}

func (dec *Decoder) Header() *Header {
	return dec.header
}

func (dec *Decoder) Decode(flvTag *tag.FlvTag) error {
	if !dec.decodedOnce {
		goto tagSize
	}

body:
	if err := tag.DecodeFlvTag(dec.r, flvTag); err != nil {
		dec.skipTagSize()
		return err
	}

tagSize:
	previousTagSize, err := dec.decodeTagSize()
	if err != nil {
		return errors.Wrap(err, "Failed to decode tag size")
	}

	if !dec.decodedOnce {
		if previousTagSize != 0 {
			return fmt.Errorf("Initial tag size should be 0: Actual = %d", previousTagSize)
		}

		dec.decodedOnce = true
		goto body
	}

	return nil
}

func (dec *Decoder) decodeTagSize() (uint32, error) {
	buffer := pool.GetBuffer()
	defer pool.PutBuffer(buffer)

	if _, err := io.CopyN(buffer, dec.r, 4); err != nil {
		return 0, err
	}

	return binary.BigEndian.Uint32(buffer.Bytes()), nil
}

func (dec *Decoder) skipTagSize() {
	lr := io.LimitReader(dec.r, 4)
	io.Copy(ioutil.Discard, lr)
}

func DecodeFlvHeader(r io.Reader) (*Header, error) {
	buffer := pool.GetBuffer()
	defer pool.PutBuffer(buffer)

	if _, err := io.CopyN(buffer, r, int64(HeaderLength)); err != nil {
		return nil, err
	}

	signature := buffer.Bytes()[0:3]
	if !bytes.Equal(signature, HeaderSignature) {
		return nil, fmt.Errorf("Signature is not matched(FLV): %+v", signature)
	}

	version := buffer.Bytes()[3]

	flags := buffer.Bytes()[4]
	//flagsReserved = (flags & 0xf8) >> 3 // 0b11111000
	flagsAudio := (flags & 0x03) >> 2 // 0b00000100
	//flagsReserved2 := (flags & 0x02) >> 1 // 0b00000010
	flagsVideo := (flags & 0x01) // 0b00000001

	dataOffset := binary.BigEndian.Uint32(buffer.Bytes()[5:9])

	header := &Header{
		Version:    version,
		DataOffset: dataOffset,
	}

	if flagsAudio != 0 {
		header.Flags |= FlagsAudio
	}
	if flagsVideo != 0 {
		header.Flags |= FlagsVideo
	}

	return header, nil
}
