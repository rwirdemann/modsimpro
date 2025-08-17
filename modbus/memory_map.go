package modbus

// MemoryMap represents a memory map for Modbus communication.
type MemoryMap struct {
	coils          map[uint16]bool
	discreteInputs map[uint16]bool
	inputRegs      map[uint16]uint16
	holdingRegs    map[uint16]uint16
}

// NewMemoryMap creates a new MemoryMap instance.
func NewMemoryMap() *MemoryMap {
	return &MemoryMap{
		coils:          make(map[uint16]bool),
		discreteInputs: make(map[uint16]bool),
		inputRegs:      make(map[uint16]uint16),
		holdingRegs:    make(map[uint16]uint16),
	}
}

// PutCoil sets the value of a coil in the memory map.
func (mm *MemoryMap) PutCoil(address uint16, value bool) {
	mm.coils[address] = value
}

// PutDiscreteInput sets the value of a discrete input in the memory map.
func (mm *MemoryMap) PutDiscreteInput(address uint16, value bool) {
	mm.discreteInputs[address] = value
}

// PutInputReg sets the value of an input register in the memory map.
func (mm *MemoryMap) PutInputReg(address uint16, value uint16) {
	mm.inputRegs[address] = value
}

func (mm MemoryMap) GetInputReg(address uint16) (uint16, bool) {
	value, ok := mm.inputRegs[address]
	return value, ok
}

// PutHoldingReg sets the value of a holding register in the memory map.
func (mm *MemoryMap) PutHoldingReg(address uint16, value uint16) {
	mm.holdingRegs[address] = value
}
