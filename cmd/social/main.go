package main

import (
	"fmt"
	"referral-system/src/social_graph/svc"
)

func main() {

	fmt.Println("This is social graph service")
	svc.RunTwitterSocialGraphService()
}
