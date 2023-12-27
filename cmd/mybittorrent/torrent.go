package main

type Torrent struct {
	Announce    string
	Length      int
	Info        map[string]any
	InfoHash    string
	PieceLength int
	Pieces      []string
}
