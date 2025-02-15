// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

//go:build generate
// +build generate

package main

import (
	"log"

	. "github.com/alimy/mir/v4/core"
	. "github.com/alimy/mir/v4/engine"
	"github.com/gin-gonic/gin"

	_ "JH-Forum/mirc/web/v1"
)

//go:generate go run $GOFILE
func main() {
	log.Println("[Mir] generate code start")
	opts := Options{
		UseGin(),
		SinkPath("auto"), // 设置生成代码的输出目录
		WatchCtxDone(true),
		RunMode(InSerialMode),
		AssertType[*gin.Context](),
	}
	if err := Generate(opts); err != nil {
		log.Fatal(err)
	}
	log.Println("[Mir] generate code finish")
}
