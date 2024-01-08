package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"

	ext_bencode "github.com/jackpal/bencode-go"
	log "github.com/sirupsen/logrus"
)

const (
	MessageTypeUnchoke    byte = 1
	MessageTypeInterested byte = 2
	MessageTypeBitfield   byte = 5
	MessageTypeRequest    byte = 6
	MessageTypePiece      byte = 7
)

const PieceBlockSize uint32 = 16384

type Torrent struct {
	Announce string "announce"
	Info     struct {
		Length      uint32 "length"
		Name        string "name"
		PieceLength uint32 "piece length"
		Pieces      string "pieces"
	} "info"
	Pieces   []string
	InfoHash string
	Peers    []string
	PeerID   string
}

type HandShake struct {
	Length   byte
	Protocol [19]byte
	Reserved [8]byte
	Hash     [20]byte
	PeerID   [20]byte
}

func NewTorrent(filePath string) (*Torrent, error) {
	var torrent Torrent
	log.Debugf("Reading torrent file %s", filePath)
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("Failed to open %s (%w)", filePath, err)
	}

	if err := ext_bencode.Unmarshal(f, &torrent); err != nil {
		return nil, fmt.Errorf("Failed to decode torrent file (%w)", err)
	}

	for i := 0; i < len(torrent.Info.Pieces); i += 20 {
		torrent.Pieces = append(torrent.Pieces, hex.EncodeToString([]byte(torrent.Info.Pieces[i:i+20])))
	}

	sha1Builder := sha1.New()
	if err = ext_bencode.Marshal(sha1Builder, torrent.Info); err != nil {
		return nil, fmt.Errorf("Failed to encode info dict (%w)", err)
	}
	torrent.InfoHash = hex.EncodeToString(sha1Builder.Sum(nil))
	torrent.PeerID = "11111111111111111111"

	if err := torrent.getPeers(); err != nil {
		return nil, fmt.Errorf("Failed to gather peers during creation")
	}
	return &torrent, nil
}

func (t *Torrent) getPeers() error {
	if len(t.Peers) != 0 {
		return nil
	}
	log.Debugf("Getting peers from %s", t.Announce)
	client := http.Client{}
	req, err := http.NewRequest("GET", t.Announce, nil)
	if err != nil {
		return fmt.Errorf("Can't initiate GET request (%w)", err)
	}
	q := req.URL.Query()
	decoded, err := hex.DecodeString(t.InfoHash)
	if err != nil {
		return fmt.Errorf("Can't decode torrent info hash (%w)", err)
	}
	q.Add("info_hash", string(decoded))
	q.Add("peer_id", t.PeerID)
	q.Add("port", "6881")
	q.Add("uploaded", "0")
	q.Add("downloaded", "0")
	q.Add("left", fmt.Sprintf("%d", t.Info.Length))
	q.Add("compact", "1")
	req.URL.RawQuery = q.Encode()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Can't request peers (%w)", err)
	}
	defer resp.Body.Close()

	data, err := ext_bencode.Decode(resp.Body)
	ipsBytes := []byte(data.(map[string]any)["peers"].(string))
	for i := 0; i < len(ipsBytes); i += 6 {
		port := int64(256)*int64(ipsBytes[i+4]) + int64(ipsBytes[i+5])
		humanIP := fmt.Sprintf("%d.%d.%d.%d:%d", ipsBytes[i], ipsBytes[i+1], ipsBytes[i+2], ipsBytes[i+3], port)
		t.Peers = append(t.Peers, humanIP)
	}
	log.Debugf("Gathered %d peers", len(t.Peers))
	return nil
}

func (t *Torrent) handshake(peer string) (net.Conn, string, error) {
	if peer == "" {
		if err := t.getPeers(); err != nil {
			return nil, "", fmt.Errorf("Failed to get peers for handshake (%w)", err)
		}
		log.Debugf("No explicit peer ID, using %s", t.Peers[0])
		peer = t.Peers[0]
	}
	log.Debugf("Establishing handshake with peer %s", peer)
	hashAsBytes, err := hex.DecodeString(t.InfoHash)
	if err != nil {
		return nil, "", fmt.Errorf("Failed to decode info hash (%w)", err)
	}
	var outgoing HandShake
	outgoing.Length = 19
	copy(outgoing.Protocol[:], []byte("BitTorrent protocol"))
	copy(outgoing.Hash[:], hashAsBytes)
	copy(outgoing.PeerID[:], []byte(t.PeerID))

	conn, err := net.Dial("tcp", peer)
	if err != nil {
		return nil, "", fmt.Errorf("Failed to establish peer connection (%w)", err)
	}

	if err := binary.Write(conn, binary.BigEndian, outgoing); err != nil {
		return nil, "", fmt.Errorf("Failed to encode struct (%w)", err)
	}

	var incoming HandShake
	binary.Read(conn, binary.BigEndian, &incoming)
	log.Debugf("Established connection with Peer ID: %s", hex.EncodeToString(incoming.PeerID[:]))

	return conn, hex.EncodeToString(incoming.PeerID[:]), nil
}

func (t *Torrent) GetPeerID(peer string) (string, error) {
	_, id, err := t.handshake(peer)
	if err != nil {
		return "", fmt.Errorf("Failed to get peer id (%w)", err)
	}
	return id, nil
}

func (t *Torrent) DownloadPiece(pieceId int, outPath string) error {
	conn, _, err := t.handshake("")
	defer conn.Close()
	if err != nil {
		return err
	}

	var b []byte
	for {
		b = make([]byte, 1024)
		n, err := conn.Read(b)
		if err != nil {
			return fmt.Errorf("Failed to read buf (%w)", err)
		}
		if n == 0 {
			break
		}
		reader := bytes.NewReader(b)
		var messageLength uint32
		var messageType byte
		if err := binary.Read(reader, binary.BigEndian, &messageLength); err != nil {
			return fmt.Errorf("Failed to read message length (%w)", err)
		}
		if err := binary.Read(reader, binary.BigEndian, &messageType); err != nil {
			return fmt.Errorf("Failed to read message type (%w)", err)
		}
		log.Debugf("Message length: %d, Message Type: %d", messageLength, messageType)
		switch messageType {
		case MessageTypeBitfield:
			if err := binary.Write(conn, binary.BigEndian, uint32(1)); err != nil {
				return fmt.Errorf("Failed to write interested message (%w)", err)
			}
			if err := binary.Write(conn, binary.BigEndian, MessageTypeInterested); err != nil {
				return fmt.Errorf("Failed to write interested message (%w)", err)
			}
		case MessageTypeUnchoke:
			if err := requestPiece(conn, t.Pieces[0], t.Info.PieceLength); err != nil {
				return fmt.Errorf("Failed to download piece (%w)", err)
			}
			return nil
		}
	}

	return nil
}

func requestPiece(conn net.Conn, piece string, pieceLength uint32) error {
	f, err := os.OpenFile("piece-0.tmp", os.O_CREATE|os.O_WRONLY, 0777)
	if err != nil {
		return fmt.Errorf("Failed to open piece for writing")
	}
	defer f.Close()

	var outbuf bytes.Buffer
	var i uint32
	for i = 0; i < pieceLength; i += PieceBlockSize {
		var requestLength = PieceBlockSize
		if pieceLength-i < requestLength {
			requestLength = pieceLength - i
		}
		log.Debugf("Requesting piece located at %d (size: %d)", i, requestLength)

		if err := binary.Write(conn, binary.BigEndian, uint32(13)); err != nil {
			return fmt.Errorf("Failed to write msg length (%w)", err)
		}
		if err := binary.Write(conn, binary.BigEndian, MessageTypeRequest); err != nil {
			return fmt.Errorf("Failed to write msg id (%w)", err)
		}

		if err := binary.Write(conn, binary.BigEndian, uint32(0)); err != nil {
			return fmt.Errorf("Failed to write piece id (%w)", err)
		}
		if err := binary.Write(conn, binary.BigEndian, uint32(i)); err != nil {
			return fmt.Errorf("Failed to write piece pos (%w)", err)
		}

		if err := binary.Write(conn, binary.BigEndian, requestLength); err != nil {
			return fmt.Errorf("Failed to write piece pos (%w)", err)
		}

		var messageLength uint32
		var messageType byte
		var pieceIndex uint32
		var pieceOffset uint32
		if err := binary.Read(conn, binary.BigEndian, &messageLength); err != nil {
			return fmt.Errorf("Failed to read message length (%w)", err)
		}
		if err := binary.Read(conn, binary.BigEndian, &messageType); err != nil {
			return fmt.Errorf("Failed to read message type (%w)", err)
		}
		if err := binary.Read(conn, binary.BigEndian, &pieceIndex); err != nil {
			return fmt.Errorf("Failed to read message type (%w)", err)
		}
		if err := binary.Read(conn, binary.BigEndian, &pieceOffset); err != nil {
			return fmt.Errorf("Failed to read message type (%w)", err)
		}
		log.Debugf("Message length: %d, Message Type: %d, Piece index: %d, Piece offset: %d",
			messageLength,
			messageType,
			pieceIndex,
			pieceOffset)

		readTotal := 0
		var blockBuffer bytes.Buffer
		for {
			b := make([]byte, 20000)
			n, err := conn.Read(b)
			if err != nil {
				return fmt.Errorf("Failed to read buf (%w)", err)
			}
			readTotal += n
			io.CopyN(&blockBuffer, bytes.NewReader(b), int64(n))
			if readTotal == int(requestLength) {
				break
			}
		}
		io.Copy(&outbuf, &blockBuffer)
	}

	sha1Builder := sha1.New()
	sha1Builder.Write(outbuf.Bytes())
	pieceHash := hex.EncodeToString(sha1Builder.Sum(nil))
	if pieceHash != piece {
		return errors.New("Failed to validate checksum")
	}

	if _, err := io.Copy(f, &outbuf); err != nil {
		return fmt.Errorf("Failed to write piece (%w)", err)
	}
	return nil
}
