package main

import (
	"fmt"
	"github.com/lynMedia/libgortsp/client"
)

func main() {
	fmt.Println("Hello libgortsp");
	var client client.RtspClient = client.RtspClient{RtspUrl: "rtsp://admin:admin1234@10.100.187.189:554/h264/ch1/Main/av_stream", DebugModel: true}
	err := client.Connect();
	if err != nil {
		fmt.Println(err)
		return
	}
	client.Options();
}