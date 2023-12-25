package main

type Torrent struct {
	Announce string `json:"announce"`
	Info     struct {
		Length int `json:"length"`
	} `json:"info"`
}
