package main

type Torrent struct {
	Announce    string
	Length      int64
	Info        map[string]any
	InfoHash    string
	PieceLength int64
	Pieces      []string
}
