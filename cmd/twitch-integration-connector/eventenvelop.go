package main

type EventEnvelop struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}
