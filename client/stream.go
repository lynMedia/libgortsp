package client
import (
	//"github.com/nareix/av"
	//"github.com/nareix/rtsp/sdp"
)
type AvStream struct {
	/*av.CodecData
	Sdp sdp.Info

	// h264
	fuBuffer []byte
	sps []byte
	pps []byte

	gotpkt bool
	pkt av.Packet
	timestamp uint32*/
}

func (self AvStream) IsAudio() bool {
	//return self.Sdp.AVType == "audio"
	return  false
}

func (self AvStream) IsVideo() bool {
	//return self.Sdp.AVType == "video"
	return  true
}