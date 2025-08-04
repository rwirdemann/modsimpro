package modbus

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/rwirdemann/modsimpro"
	"github.com/simonvetter/modbus"
)

type Adapter struct {
	client *modbus.ModbusClient
}

func NewAdapter(serial modsimpro.Serial) Adapter {
	client, err := modbus.NewClient(&modbus.ClientConfiguration{
		URL:      serial.Url,
		Speed:    uint(serial.Speed),
		DataBits: uint(serial.DataBits),
		Parity:   uint(serial.Parity),
		StopBits: uint(serial.StopBits),
		Timeout:  time.Duration(serial.Timeout) * time.Millisecond,
	})
	if err != nil {
		panic(err)
	}
	if err = client.Open(); err != nil {
		panic(err)
	}

	return Adapter{client: client}
}

func (a Adapter) Close() {
	_ = a.client.Close()
}

func (a Adapter) ReadRegister(register []modsimpro.Register) []modsimpro.Register {
	var rr []modsimpro.Register
	for _, r := range register {
		switch r.RegisterType {
		case "holding":
			holding, err := a.readHolding(r)
			if err != nil {
				slog.Error("error reading holding register", "err", err)
				continue
			}
			rr = append(rr, holding)
		case "input":
			input, err := a.readInput(r)
			if err != nil {
				slog.Error("error reading input register", "err", err)
				continue
			}
			rr = append(rr, input)
		case "discrete":
			discrete, err := a.readDiscrete(r)
			if err != nil {
				slog.Error("error reading discrete register", "err", err)
				continue
			}
			rr = append(rr, discrete)
		default:
			slog.Error("unknown register type", "type", r.RegisterType)
		}
	}
	return rr
}

func (a Adapter) WriteRegister(r modsimpro.Register) error {
	if err := a.client.SetUnitId(r.SlaveAddress); err != nil {
		return fmt.Errorf("set unit id: %w", err)
	}

	switch r.Datatype {
	case "BOOL":
		v := r.RawData.(bool)
		if err := a.client.WriteCoil(r.Address, v); err != nil {
			return err
		}
	case "F32T1234":
		v := r.RawData.(float32)
		if err := a.client.WriteFloat32(r.Address, v); err != nil {
			return err
		}
	case "F32T3412":
		buf := new(bytes.Buffer)
		err := binary.Write(buf, binary.BigEndian, r.RawData)
		if err != nil {
			return err
		}
		byteArray := buf.Bytes()
		msb := binary.BigEndian.Uint16(byteArray[0:])
		lsb := binary.BigEndian.Uint16(byteArray[2:])
		bits := (uint32(lsb) << 16) | uint32(msb)
		v := math.Float32frombits(bits)
		if err := a.client.WriteFloat32(r.Address, v); err != nil {
			return err
		}
	case "T64T1234":
		v := r.RawData.(uint64)
		if err := a.client.WriteUint64(r.Address, v); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown datatype: %s", r.Datatype)
	}
	return nil
}

func (a Adapter) readHolding(register modsimpro.Register) (modsimpro.Register, error) {
	if err := a.client.SetUnitId(register.SlaveAddress); err != nil {
		return modsimpro.Register{}, fmt.Errorf("set unit id: %w", err)
	}

	switch register.Datatype {
	case "F32T1234":
		v, err := a.client.ReadFloat32(register.Address, modbus.HOLDING_REGISTER)
		if err != nil {
			return modsimpro.Register{}, err
		}
		register.RawData = v
		return register, nil
	case "F32T3412":
		bb, err := a.client.ReadRawBytes(register.Address, 4, modbus.HOLDING_REGISTER)
		if err != nil {
			return modsimpro.Register{}, err
		}
		msb := binary.BigEndian.Uint16(bb[0:2])
		lsb := binary.BigEndian.Uint16(bb[2:4])
		bits := (uint32(lsb) << 16) | uint32(msb)
		register.RawData = math.Float32frombits(bits)
		return register, nil
	default:
		return modsimpro.Register{}, fmt.Errorf("unknown datatype: %s", register.Datatype)
	}
}

func (a Adapter) readInput(register modsimpro.Register) (modsimpro.Register, error) {
	if err := a.client.SetUnitId(register.SlaveAddress); err != nil {
		return modsimpro.Register{}, fmt.Errorf("set unit id: %w", err)
	}

	switch register.Datatype {
	case "F32T1234", "F32T3412":
		v, err := a.client.ReadFloat32(register.Address, modbus.INPUT_REGISTER)
		if err != nil {
			return modsimpro.Register{}, err
		}
		register.RawData = v
		return register, nil
	case "T64T1234":
		v, err := a.client.ReadUint64(register.Address, modbus.INPUT_REGISTER)
		if err != nil {
			return modsimpro.Register{}, err
		}
		register.RawData = v
		return register, nil
	default:
		return modsimpro.Register{}, fmt.Errorf("unknown datatype: %s", register.Datatype)
	}
}

func (a Adapter) readDiscrete(register modsimpro.Register) (modsimpro.Register, error) {
	if err := a.client.SetUnitId(register.SlaveAddress); err != nil {
		return modsimpro.Register{}, fmt.Errorf("set unit id: %w", err)
	}

	b, err := a.client.ReadDiscreteInput(register.Address)
	if err != nil {
		return modsimpro.Register{}, err
	}
	register.RawData = b
	return register, nil
}
