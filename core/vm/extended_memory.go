package vm

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/holiman/uint256"
)

const EXT_DEFAULT_MAX_GAS = 1_000_000_000_000

// Ext_getData checks memory gas cost before padding bytes
func Ext_getData(data []byte, start uint64, size uint64, maxGas uint64) ([]byte, error) {
	gas, err := memoryGasCost(NewMemory(), size)

	if err != nil {
		return nil, err
	}

	if gas > maxGas {
		return nil, ErrOutOfGas
	}

	length := uint64(len(data))
	if start > length {
		start = length
	}
	end := start + size
	if end > length {
		end = length
	}
	return common.RightPadBytes(data[start:end], int(size)), nil
}

// Ext_Set sets offset + size to value, with auto resizing
func (m *Memory) Ext_Set(offset, size uint64, value []byte, maxGas uint64) error {
	// It's possible the offset is greater than 0 and size equals 0. This is because
	// the calcMemSize (common.go) could potentially return 0 when size is zero (NO-OP)
	if size > 0 {
		err := m.Ext_Resize(offset+size, maxGas)
		if err != nil {
			return err
		}
		copy(m.store[offset:offset+size], value)
	}
	return nil
}

// Ext_Set32 with auto resizing
func (m *Memory) Ext_Set32(offset uint64, val *uint256.Int, maxGas uint64) error {
	err := m.Ext_Resize(offset+32, maxGas)
	if err != nil {
		return err
	}
	// Fill in relevant bits
	b32 := val.Bytes32()
	copy(m.store[offset:], b32[:])
	return nil
}

// Ext_Set1 set one byte (for mstore8) with auto resizing
func (m *Memory) Ext_Set1(offset uint64, val byte, maxGas uint64) error {
	err := m.Ext_Resize(offset+1, maxGas)
	if err != nil {
		return err
	}
	m.store[offset] = val
	return nil
}

// Ext_GetCopy is GetCopy with auto resizing
func (m *Memory) Ext_GetCopy(offset, size int64, maxGas uint64) ([]byte, error) {
	err := m.Ext_Resize(uint64(offset+size), maxGas)
	if err != nil {
		return nil, err
	}
	return m.GetCopy(offset, size), nil
}

// Ext_GetPtr is GetPtr with auto resizing
func (m *Memory) Ext_GetPtr(offset, size int64, maxGas uint64) ([]byte, error) {
	err := m.Ext_Resize(uint64(offset+size), maxGas)
	if err != nil {
		return nil, err
	}
	return m.GetPtr(offset, size), nil
}

// Ext_Resize checks constant memory limit before resizing.
func (m *Memory) Ext_Resize(newMemSize uint64, maxGas uint64) error {
	gas, err := memoryGasCost(m, newMemSize)

	if err != nil {
		return err
	}

	if gas > maxGas {
		return ErrOutOfGas
	}

	var memorySize uint64
	memSize, overflow := newMemSize, false
	// memory is expanded in words of 32 bytes. Gas
	// is also calculated in words.
	if memorySize, overflow = math.SafeMul(toWordSize(memSize), 32); overflow {
		return ErrGasUintOverflow
	}

	if memorySize > 0 {
		m.Resize(memorySize)
	}

	return nil
}

// Ext_Dump dumps the content of the memory.
func (m *Memory) Ext_Dump() {
	fmt.Printf("### mem %d bytes ###\n", len(m.store))
	if len(m.store) > 0 {
		addr := 0
		for i := 0; i+32 <= len(m.store); i += 32 {
			fmt.Printf("%04x: % x\n", addr*32, m.store[i:i+32])
			addr++
		}
	} else {
		fmt.Println("-- empty --")
	}
	fmt.Println("####################")
}
