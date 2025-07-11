package modsimpro

import (
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net"
	"strings"
	"time"
)

type Logger interface {
	Append(text string)
}

// ModbusServer represents a TCP based modbus server with multiple slaves connected to it.
type ModbusServer struct {
	url         string
	logger      Logger
	tcpListener net.Listener
	sock        net.Conn
	lastTxnId   uint16
	slaves      map[int]bool
}

func NewModbusServer(url string, logger Logger) *ModbusServer {
	splitURL := strings.SplitN(url, "://", 2)
	if len(splitURL) == 2 {
		return &ModbusServer{url: splitURL[1], logger: logger, slaves: make(map[int]bool)}
	}
	return nil
}

func (s *ModbusServer) Start() (err error) {
	s.tcpListener, err = net.Listen("tcp", s.url)
	if err == nil {
		go s.acceptTCPClients()
	}
	return
}

func (s *ModbusServer) Connect(slaveID int) {
	s.slaves[slaveID] = true
}

func (s *ModbusServer) Disconnect(slaveID int) {
	s.slaves[slaveID] = false
}

func (s *ModbusServer) acceptTCPClients() {
	for {
		var err error
		s.sock, err = s.tcpListener.Accept()
		if err != nil {
			slog.Warn("failed to accept client connection: %v", err)
			continue
		}
		ts := time.Now().Format(time.DateTime)
		text := fmt.Sprintf("%s: client %s connected", ts, s.sock.RemoteAddr())
		s.logger.Append(text)
		go s.handleClient()

		// sock.Close()
	}
}

type Endianness uint
type Error string

const (
	fcReadDiscreteInputs uint8 = 0x02
	mbapHeaderLength     int   = 7

	// endianness of 16-bit registers
	BIG_ENDIAN        Endianness = 1
	LITTLE_ENDIAN     Endianness = 2
	maxTCPFrameLength int        = 260

	ErrProtocolError     Error = "protocol error"
	ErrUnknownProtocolId Error = "unknown protocol identifier"
)

// Error implements the error interface.
func (me Error) Error() (s string) {
	s = string(me)
	return
}

type pdu struct {
	unitId       uint8
	functionCode uint8
	payload      []byte
}

func (s *ModbusServer) handleClient() (req *pdu, err error) {
	var txnId uint16
	for {
		req, txnId, err = s.readMBAPFrame()
		if err != nil || req == nil {
			continue
		}
		ts := time.Now().Format(time.DateTime)
		s.logger.Append(fmt.Sprintf("%s req: slave id: %d fc: %X payload: % X", ts, req.unitId, req.functionCode, req.payload))

		// store the incoming transaction id
		s.lastTxnId = txnId

		if !s.slaves[int(req.unitId)] {
			ts := time.Now().Format(time.DateTime)
			s.logger.Append(fmt.Sprintf("%s req: slave id: %d is offline", ts, req.unitId))
			continue
		}

		if req.functionCode == fcReadDiscreteInputs {
			// addr := bytesToUint16(BIG_ENDIAN, req.payload[0:2])
			quantity := bytesToUint16(BIG_ENDIAN, req.payload[2:4])

			var values = make([]bool, quantity)
			for i := range int(quantity) {
				values[i] = rand.Intn(2) == 1
			}
			resCount := len(values)

			// assemble a response PDU
			res := &pdu{
				unitId:       req.unitId,
				functionCode: req.functionCode,
				payload:      []byte{0},
			}
			// byte count (1 byte for 8 coils)
			res.payload[0] = uint8(resCount / 8)
			if resCount%8 != 0 {
				res.payload[0]++
			}

			// coil values
			res.payload = append(res.payload, encodeBools(values)...)

			ts := time.Now().Format(time.DateTime)
			s.logger.Append(fmt.Sprintf("%s res: slave id: %d fc: %X payload: % X", ts, res.unitId, res.functionCode, res.payload))

			_, err = s.sock.Write(s.assembleMBAPFrame(s.lastTxnId, res))
			if err != nil {
				return
			}
		}
	}
}

// Reads an entire frame (MBAP header + modbus PDU) from the socket.
func (s *ModbusServer) readMBAPFrame() (p *pdu, txnId uint16, err error) {
	var rxbuf []byte
	var bytesNeeded int
	var protocolId uint16
	var unitId uint8

	// read the MBAP header
	rxbuf = make([]byte, mbapHeaderLength)
	_, err = io.ReadFull(s.sock, rxbuf)
	if err != nil {
		return
	}

	// decode the transaction identifier
	txnId = bytesToUint16(BIG_ENDIAN, rxbuf[0:2])
	// decode the protocol identifier
	protocolId = bytesToUint16(BIG_ENDIAN, rxbuf[2:4])
	// store the source unit id
	unitId = rxbuf[6]

	// determine how many more bytes we need to read
	bytesNeeded = int(bytesToUint16(BIG_ENDIAN, rxbuf[4:6]))

	// the byte count includes the unit ID field, which we already have
	bytesNeeded--

	// never read more than the max allowed frame length
	if bytesNeeded+mbapHeaderLength > maxTCPFrameLength {
		err = ErrProtocolError
		return
	}

	// an MBAP length of 0 is illegal
	if bytesNeeded <= 0 {
		err = ErrProtocolError
		return
	}

	// read the PDU
	rxbuf = make([]byte, bytesNeeded)
	_, err = io.ReadFull(s.sock, rxbuf)
	if err != nil {
		return
	}

	// validate the protocol identifier
	if protocolId != 0x0000 {
		err = ErrUnknownProtocolId
		slog.Warn("received unexpected protocol id 0x%04x", protocolId)
		return
	}

	// store unit id, function code and payload in the PDU object
	p = &pdu{
		unitId:       unitId,
		functionCode: rxbuf[0],
		payload:      rxbuf[1:],
	}

	return
}

// Turns a PDU into an MBAP frame (MBAP header + PDU) and returns it as bytes.
func (s *ModbusServer) assembleMBAPFrame(txnId uint16, p *pdu) (payload []byte) {
	// transaction identifier
	payload = uint16ToBytes(BIG_ENDIAN, txnId)
	// protocol identifier (always 0x0000)
	payload = append(payload, 0x00, 0x00)
	// length (covers unit identifier + function code + payload fields)
	payload = append(payload, uint16ToBytes(BIG_ENDIAN, uint16(2+len(p.payload)))...)
	// unit identifier
	payload = append(payload, p.unitId)
	// function code
	payload = append(payload, p.functionCode)
	// payload
	payload = append(payload, p.payload...)

	return
}

func bytesToUint16(endianness Endianness, in []byte) (out uint16) {
	switch endianness {
	case BIG_ENDIAN:
		out = binary.BigEndian.Uint16(in)
	case LITTLE_ENDIAN:
		out = binary.LittleEndian.Uint16(in)
	}

	return
}

func uint16ToBytes(endianness Endianness, in uint16) (out []byte) {
	out = make([]byte, 2)
	switch endianness {
	case BIG_ENDIAN:
		binary.BigEndian.PutUint16(out, in)
	case LITTLE_ENDIAN:
		binary.LittleEndian.PutUint16(out, in)
	}

	return
}

func encodeBools(in []bool) (out []byte) {
	var byteCount uint
	var i uint

	byteCount = uint(len(in)) / 8
	if len(in)%8 != 0 {
		byteCount++
	}

	out = make([]byte, byteCount)
	for i = range uint(len(in)) {
		if in[i] {
			out[i/8] |= (0x01 << (i % 8))
		}
	}

	return
}
