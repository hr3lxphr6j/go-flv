//
// Copyright (c) 2018- yutopp (yutopp@gmail.com)
//
// Distributed under the Boost Software License, Version 1.0. (See accompanying
// file LICENSE_1_0.txt or copy at  https://www.boost.org/LICENSE_1_0.txt)
//

package tag

import (
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/pkg/errors"
	"github.com/yutopp/go-amf0"

	"github.com/yutopp/go-flv/pool"
)

func DecodeFlvTag(r io.Reader, flvTag *FlvTag) (err error) {
	buffer := pool.GetBuffer()
	defer pool.PutBuffer(buffer)

	if _, err := io.CopyN(buffer, r, 11); err != nil {
		return err
	}

	tagType := TagType(buffer.Bytes()[0])
	dataSize := uint32(buffer.Bytes()[1])<<16 | uint32(buffer.Bytes()[2])<<8 | uint32(buffer.Bytes()[3])
	timestamp := uint32(buffer.Bytes()[4])<<16 | uint32(buffer.Bytes()[5])<<8 | uint32(buffer.Bytes()[6]) | uint32(buffer.Bytes()[7])<<24
	streamID := uint32(buffer.Bytes()[8])<<16 | uint32(buffer.Bytes()[9])<<8 | uint32(buffer.Bytes()[10])

	*flvTag = FlvTag{
		TagType:   tagType,
		Timestamp: timestamp,
		StreamID:  streamID,
	}

	lr := io.LimitReader(r, int64(dataSize))
	defer func() {
		if err != nil {
			_, _ = io.Copy(ioutil.Discard, lr) // TODO: wrap an error?
		}
	}()

	switch tagType {
	case TagTypeAudio:
		var v AudioData
		if err := DecodeAudioData(lr, &v); err != nil {
			return errors.Wrap(err, "Failed to decode audio data")
		}
		flvTag.Data = &v

	case TagTypeVideo:
		var v VideoData
		if err := DecodeVideoData(lr, &v); err != nil {
			return errors.Wrap(err, "Failed to decode video data")
		}
		flvTag.Data = &v

	case TagTypeScriptData:
		var v ScriptData
		if err := DecodeScriptData(lr, &v); err != nil {
			return errors.Wrap(err, "Failed to decode script data")
		}
		flvTag.Data = &v

	default:
		return fmt.Errorf("Unsupported tag type: %+v", tagType)
	}

	return nil
}

func DecodeAudioData(r io.Reader, audioData *AudioData) error {
	buffer := pool.GetBuffer()
	defer pool.PutBuffer(buffer)
	if _, err := io.CopyN(buffer, r, 1); err != nil {
		return err
	}

	soundFormat := SoundFormat(buffer.Bytes()[0] & 0xf0 >> 4) // 0b11110000
	soundRate := SoundRate(buffer.Bytes()[0] & 0x0c >> 2)     // 0b00001100
	soundSize := SoundSize(buffer.Bytes()[0] & 0x02 >> 1)     // 0b00000010
	soundType := SoundType(buffer.Bytes()[0] & 0x01)          // 0b00000001

	*audioData = AudioData{
		SoundFormat: soundFormat,
		SoundRate:   soundRate,
		SoundSize:   soundSize,
		SoundType:   soundType,
	}

	if soundFormat == SoundFormatAAC {
		var aacAudioData AACAudioData
		if err := DecodeAACAudioData(r, &aacAudioData); err != nil {
			return wrapEOF(err)
		}

		audioData.AACPacketType = aacAudioData.AACPacketType
		audioData.Data = aacAudioData.Data
	} else {
		audioData.Data = r
	}

	return nil
}

func DecodeAACAudioData(r io.Reader, aacAudioData *AACAudioData) error {
	buffer := pool.GetBuffer()
	defer pool.PutBuffer(buffer)
	if _, err := io.CopyN(buffer, r, 1); err != nil {
		return err
	}

	aacPacketType := AACPacketType(buffer.Bytes()[0])

	*aacAudioData = AACAudioData{
		AACPacketType: aacPacketType,
		Data:          r,
	}

	return nil
}

func DecodeVideoData(r io.Reader, videoData *VideoData) error {
	buffer := pool.GetBuffer()
	defer pool.PutBuffer(buffer)
	if _, err := io.CopyN(buffer, r, 1); err != nil {
		return err
	}

	frameType := FrameType(buffer.Bytes()[0] & 0xf0 >> 4) // 0b11110000
	codecID := CodecID(buffer.Bytes()[0] & 0x0f)          // 0b00001111

	*videoData = VideoData{
		FrameType: frameType,
		CodecID:   codecID,
	}

	if codecID == CodecIDAVC {
		var avcVideoPacket AVCVideoPacket
		if err := DecodeAVCVideoPacket(r, &avcVideoPacket); err != nil {
			return wrapEOF(err)
		}
		videoData.AVCPacketType = avcVideoPacket.AVCPacketType
		videoData.CompositionTime = avcVideoPacket.CompositionTime
		videoData.Data = avcVideoPacket.Data
	} else {
		videoData.Data = r
	}

	return nil
}

func DecodeAVCVideoPacket(r io.Reader, avcVideoPacket *AVCVideoPacket) error {
	buffer := pool.GetBuffer()
	defer pool.PutBuffer(buffer)
	if _, err := io.CopyN(buffer, r, 4); err != nil {
		return err
	}

	avcPacketType := AVCPacketType(buffer.Bytes()[0])
	compositionTime := int32(binary.BigEndian.Uint32(buffer.Bytes()[1:])) >> 8 // Signed Interger 24 bits. TODO: check

	*avcVideoPacket = AVCVideoPacket{
		AVCPacketType:   avcPacketType,
		CompositionTime: compositionTime,
		Data:            r,
	}

	return nil
}

func DecodeScriptData(r io.Reader, data *ScriptData) error {
	dec := amf0.NewDecoder(r)

	kv := make(map[string]amf0.ECMAArray)
	for {
		var key string
		if err := dec.Decode(&key); err != nil {
			if err == io.EOF {
				break
			}
			return errors.Wrap(err, "Failed to decode key")
		}

		var value amf0.ECMAArray
		if err := dec.Decode(&value); err != nil {
			return errors.Wrap(err, "Failed to decode value")
		}

		kv[key] = value
	}

	data.Objects = kv

	return nil
}

func wrapEOF(err error) error {
	if err == io.EOF {
		return io.ErrUnexpectedEOF
	}
	return err
}
