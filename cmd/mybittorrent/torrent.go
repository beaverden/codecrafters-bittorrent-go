package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/rand"
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
	log.Debugf("Torrent length: %d", torrent.Info.Length)
	log.Debugf("Torrent piece length: %d", torrent.Info.PieceLength)
	log.Debugf("Torrent nr pieces: %d", len(torrent.Pieces))
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
	if data.(map[string]any)["peers"] == nil {
		log.Debugf("No peers found")
		return nil
	}

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
		peer = t.Peers[rand.Intn(len(t.Peers))]
		log.Debugf("No explicit peer ID, using %s", peer)
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

func (t *Torrent) DownloadFile(outPath string) error {
	f, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0777)
	if err != nil {
		return fmt.Errorf("Failed to open piece for writing")
	}
	defer f.Close()

	for pieceId, _ := range t.Pieces {
		if err := t.downloadPiece(pieceId, f); err != nil {
			return fmt.Errorf("Failed to download piece %d (%w)", pieceId, err)
		}
	}
	return nil
}

func (t *Torrent) DownloadPiece(pieceId int, outPath string) error {
	f, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0777)
	if err != nil {
		return fmt.Errorf("Failed to open piece for writing")
	}
	defer f.Close()
	if err := t.downloadPiece(pieceId, f); err != nil {
		return fmt.Errorf("Failed to download piece (%w)", err)
	}
	f.Close()
	return nil
}

func (t *Torrent) downloadPiece(pieceId int, out io.Writer) error {
	conn, _, err := t.handshake("")
	defer conn.Close()
	if err != nil {
		return err
	}
	var pieceBuf bytes.Buffer

	requestedBlocks := 0
msgLoop:
	for {
		var messageLength uint32
		var messageType byte
		if err := binary.Read(conn, binary.BigEndian, &messageLength); err != nil {
			return fmt.Errorf("Failed to read message length (%w)", err)
		}
		if err := binary.Read(conn, binary.BigEndian, &messageType); err != nil {
			return fmt.Errorf("Failed to read message type (%w)", err)
		}

		log.Debugf("Message length: %d, Message Type: %d", messageLength, messageType)
		switch messageType {
		case MessageTypeBitfield:
			log.Debug("Found BITFIELD")
			payload := make([]byte, messageLength-1)
			if err := binary.Read(conn, binary.BigEndian, &payload); err != nil {
				return fmt.Errorf("Failed to read bitfield (%w)", err)
			}
			if err := binary.Write(conn, binary.BigEndian, uint32(1)); err != nil {
				return fmt.Errorf("Failed to write interested message (%w)", err)
			}
			if err := binary.Write(conn, binary.BigEndian, MessageTypeInterested); err != nil {
				return fmt.Errorf("Failed to write interested message (%w)", err)
			}

		case MessageTypeUnchoke:
			log.Debug("FOUND UNCHOKE")
			pieceLength := t.Info.PieceLength
			if pieceId == len(t.Pieces)-1 {
				pieceLength = t.Info.Length % t.Info.PieceLength
			}

			nrBlocks := (pieceLength + PieceBlockSize - 1) / PieceBlockSize
			log.Debugf("Dividing piece length %d into %d blocks", t.Info.PieceLength, nrBlocks)
			var i uint32
			for i = 0; i < pieceLength; i += PieceBlockSize {
				var requestLength = PieceBlockSize
				if i+PieceBlockSize > pieceLength {
					requestLength = pieceLength % PieceBlockSize
				}
				log.Debugf("Requesting block %d (size: %d)", i, requestLength)
				if err := binary.Write(conn, binary.BigEndian, uint32(13)); err != nil {
					return fmt.Errorf("Failed to write msg length (%w)", err)
				}
				if err := binary.Write(conn, binary.BigEndian, MessageTypeRequest); err != nil {
					return fmt.Errorf("Failed to write msg id (%w)", err)
				}

				if err := binary.Write(conn, binary.BigEndian, uint32(pieceId)); err != nil {
					return fmt.Errorf("Failed to write piece id (%w)", err)
				}
				if err := binary.Write(conn, binary.BigEndian, uint32(i)); err != nil {
					return fmt.Errorf("Failed to write piece pos (%w)", err)
				}

				if err := binary.Write(conn, binary.BigEndian, requestLength); err != nil {
					return fmt.Errorf("Failed to write piece pos  (%w)", err)
				}
				requestedBlocks += 1
			}
		case MessageTypePiece:
			log.Debug("FOUND PIECE")
			var pieceIndex uint32
			var pieceOffset uint32
			if err := binary.Read(conn, binary.BigEndian, &pieceIndex); err != nil {
				return fmt.Errorf("Failed to read message type (%w)", err)
			}
			if err := binary.Read(conn, binary.BigEndian, &pieceOffset); err != nil {
				return fmt.Errorf("Failed to read message type (%w)", err)
			}
			data := make([]byte, messageLength-9)
			if err := binary.Read(conn, binary.BigEndian, &data); err != nil {
				return fmt.Errorf("Failed to read block data (%w)", err)
			}
			if _, err = pieceBuf.Write(data); err != nil {
				return fmt.Errorf("Failed to write block to file (%w)", err)
			}
			requestedBlocks -= 1
			if requestedBlocks == 0 {
				log.Debug("Downloaded all the pieces")
				break msgLoop
			}
		default:
			return errors.New(fmt.Sprintf("Unknown message type: %d", messageType))
		}
	}

	h := sha1.New()
	h.Write(pieceBuf.Bytes())
	pieceHash := hex.EncodeToString(h.Sum(nil))
	if pieceHash != t.Pieces[pieceId] {
		return errors.New(fmt.Sprintf("Hash invalid [%s] (required: %s)", pieceHash, t.Pieces[pieceId]))
	}
	_, err = io.Copy(out, &pieceBuf)
	if err != nil {
		return fmt.Errorf("Failed to copy piece buf to out buf (%w)", err)
	}
	return nil
}
