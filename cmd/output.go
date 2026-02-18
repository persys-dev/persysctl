package cmd

import (
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func printProto(msg proto.Message) {
	if msg == nil {
		fmt.Println("{}")
		return
	}
	b, err := protojson.MarshalOptions{Indent: "  "}.Marshal(msg)
	if err != nil {
		fmt.Println("{}")
		return
	}
	fmt.Println(string(b))
}
