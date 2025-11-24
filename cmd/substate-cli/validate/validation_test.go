package validate

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/research"
	"github.com/holiman/uint256"
	"google.golang.org/protobuf/proto"
)

func A(hex string) []byte {
	v := common.HexToAddress(hex)
	return research.AddressToBytes(&v)
}

func U(hex string) []byte {
	v := uint256.MustFromBig(common.HexToHash(hex).Big())
	return research.Uint256ToBytes(v)
}

func B(hex string) []byte {
	return common.Hex2Bytes(hex)
}

func H(hex string) []byte {
	v := common.HexToHash(hex)
	return research.HashToBytes(&v)
}

func newTestSubstate() *research.Substate {
	x := &research.Substate{
		InputAlloc: &research.Substate_Alloc{},
		BlockEnv: &research.Substate_BlockEnv{
			Number: proto.Uint64(1_234_567),
		},
		OutputAlloc: &research.Substate_Alloc{},
	}
	x.InputAlloc.Alloc = []*research.Substate_AllocEntry{
		{
			Address: A("0x11"),
			Account: &research.Substate_Account{
				Nonce:   proto.Uint64(0x11),
				Balance: U("0x11"),
				Contract: &research.Substate_Account_Code{
					Code: B("0x11"),
				},
				Storage: []*research.Substate_Account_StorageEntry{
					{Key: H("0x11"), Value: H("0x11")},
					{Key: H("0x22"), Value: H("0x22")},
				},
			},
		},
	}
	x.OutputAlloc.Alloc = []*research.Substate_AllocEntry{
		{
			Address: A("0x11"),
			Account: &research.Substate_Account{
				Nonce:   proto.Uint64(0x12),
				Balance: U("0x12"),
				Contract: &research.Substate_Account_Code{
					Code: B("0x11"),
				},
				Storage: []*research.Substate_Account_StorageEntry{
					{Key: H("0x11"), Value: H("0x11")},
					{Key: H("0x22"), Value: H("0xffff")},
				},
			},
		},
	}
	return x
}

func TestEquivalentAllocAccountInfo(t *testing.T) {
	x := newTestSubstate()

	y := proto.Clone(x).(*research.Substate)
	*y.OutputAlloc.Alloc[0].Account.Nonce++

	var eqAlloc bool

	eqAlloc, _ = EquivalentAlloc(x, y)
	if eqAlloc {
		t.Errorf("eqAlloc must be false for different account info")
	}

	eqAlloc, _ = EquivalentAlloc(y, x)
	if eqAlloc {
		t.Errorf("eqAlloc must be false for different account info")
	}
}

func TestEquivalentAllocSSTORE1(t *testing.T) {
	x := newTestSubstate()

	y := proto.Clone(x).(*research.Substate)
	y.OutputAlloc.Alloc[0].Account.Storage = []*research.Substate_Account_StorageEntry{
		{Key: H("0x11"), Value: H("0xffff")},
		{Key: H("0x22"), Value: H("0xffff")},
	}

	var eqAlloc, matchedSA bool

	eqAlloc, matchedSA = EquivalentAlloc(x, y)
	if eqAlloc {
		t.Errorf("eqAlloc must be false for different storage values")
	}
	if !matchedSA {
		t.Errorf("matchedSA must be true for same storage keys")
	}

	eqAlloc, matchedSA = EquivalentAlloc(y, x)
	if eqAlloc {
		t.Errorf("eqAlloc must be false for different storage values")
	}
	if !matchedSA {
		t.Errorf("matchedSA must be true for same storage keys")
	}
}

func TestEquivalentAllocSSTORE2(t *testing.T) {
	x := newTestSubstate()

	y := proto.Clone(x).(*research.Substate)
	y.InputAlloc.Alloc[0].Account.Storage = []*research.Substate_Account_StorageEntry{
		{Key: H("0x11"), Value: H("0x11")},
		{Key: H("0x22"), Value: H("0xffff")},
		{Key: H("0x33"), Value: H("0x00")},
	}
	y.OutputAlloc.Alloc[0].Account.Storage = []*research.Substate_Account_StorageEntry{
		{Key: H("0x11"), Value: H("0x11")},
		{Key: H("0x22"), Value: H("0xffff")},
		{Key: H("0x33"), Value: H("0xffff")},
	}

	var eqAlloc, matchedSA bool

	eqAlloc, matchedSA = EquivalentAlloc(x, y)
	if eqAlloc {
		t.Errorf("eqAlloc must be false for different storage values")
	}
	if matchedSA {
		t.Errorf("matchedSA must be false for different storage keys")
	}

	eqAlloc, matchedSA = EquivalentAlloc(y, x)
	if eqAlloc {
		t.Errorf("eqAlloc must be false for different storage values")
	}
	if matchedSA {
		t.Errorf("matchedSA must be false for different storage keys")
	}
}

func TestEquivalentAllocRedundantSLOAD(t *testing.T) {
	x := newTestSubstate()

	y := proto.Clone(x).(*research.Substate)
	y.InputAlloc.Alloc[0].Account.Storage = []*research.Substate_Account_StorageEntry{
		{Key: H("0x11"), Value: H("0x11")},
		{Key: H("0x22"), Value: H("0x22")},
		{Key: H("0x33"), Value: H("0x00")},
	}
	y.OutputAlloc.Alloc[0].Account.Storage = []*research.Substate_Account_StorageEntry{
		{Key: H("0x11"), Value: H("0x11")},
		{Key: H("0x22"), Value: H("0xffff")},
		{Key: H("0x33"), Value: H("0x00")},
	}

	var eqAlloc, matchedSA bool

	eqAlloc, matchedSA = EquivalentAlloc(x, y)
	if !eqAlloc {
		t.Errorf("eqAlloc must be true for redundant SLOAD with same side effects")
	}
	if matchedSA {
		t.Errorf("matchedSA must be false for different storage keys")
	}

	eqAlloc, matchedSA = EquivalentAlloc(y, x)
	if !eqAlloc {
		t.Errorf("eqAlloc must be true for redundant SLOAD with same side effects")
	}
	if matchedSA {
		t.Errorf("matchedSA must be false for different storage keys")
	}
}

func TestEquivalentAllocInsufficientSLOAD(t *testing.T) {
	x := newTestSubstate()

	y := proto.Clone(x).(*research.Substate)
	y.InputAlloc.Alloc[0].Account.Storage = []*research.Substate_Account_StorageEntry{
		{Key: H("0x22"), Value: H("0x22")},
	}
	y.OutputAlloc.Alloc[0].Account.Storage = []*research.Substate_Account_StorageEntry{
		{Key: H("0x22"), Value: H("0xffff")},
	}

	var eqAlloc, matchedSA bool

	eqAlloc, matchedSA = EquivalentAlloc(x, y)
	if !eqAlloc {
		t.Errorf("eqAlloc must be true for insufficient SLOAD with same side effects")
	}
	if matchedSA {
		t.Errorf("matchedSA must be false for different storage keys")
	}

	eqAlloc, matchedSA = EquivalentAlloc(y, x)
	if !eqAlloc {
		t.Errorf("eqAlloc must be true for insufficient SLOAD with same side effects")
	}
	if matchedSA {
		t.Errorf("matchedSA must be false for different storage keys")
	}
}

func TestEquivalentAllocAccountDeletion1(t *testing.T) {
	x := newTestSubstate()
	x.OutputAlloc.Alloc = nil

	y := proto.Clone(x).(*research.Substate)
	y.InputAlloc.Alloc = nil
	y.OutputAlloc.Alloc = nil

	var eqAlloc bool

	eqAlloc, _ = EquivalentAlloc(x, y)
	if eqAlloc {
		t.Errorf("eqAlloc must be false for missing account deletion")
	}

	eqAlloc, _ = EquivalentAlloc(y, x)
	if eqAlloc {
		t.Errorf("eqAlloc must be false for missing account deletion")
	}
}

func TestEquivalentAllocAccountDeletion2(t *testing.T) {
	x := newTestSubstate()
	x.OutputAlloc.Alloc = nil

	y := proto.Clone(x).(*research.Substate)
	y.OutputAlloc = proto.Clone(y.InputAlloc).(*research.Substate_Alloc)

	eqAlloc, _ := EquivalentAlloc(x, y)
	if eqAlloc {
		t.Errorf("eqAlloc must be false for missing account deletion")
	}
}

func TestEquivalentAllocAccountDeletion3(t *testing.T) {
	x := newTestSubstate()

	y := proto.Clone(x).(*research.Substate)
	y.OutputAlloc.Alloc = nil

	var eqAlloc bool

	eqAlloc, _ = EquivalentAlloc(x, y)
	if eqAlloc {
		t.Errorf("eqAlloc must be false for wrong account deletion")
	}

	eqAlloc, _ = EquivalentAlloc(y, x)
	if eqAlloc {
		t.Errorf("eqAlloc must be false for wrong account deletion")
	}
}

func TestEquivalentAllocAccountDeletion4(t *testing.T) {
	s := newTestSubstate()
	s.InputAlloc.Alloc = append(
		s.InputAlloc.Alloc,
		&research.Substate_AllocEntry{
			Address: A("0x22"),
			Account: &research.Substate_Account{
				Nonce:   proto.Uint64(0x22),
				Balance: U("0x22"),
			},
		},
	)
	s.OutputAlloc.Alloc = append(
		s.OutputAlloc.Alloc,
		&research.Substate_AllocEntry{
			Address: A("0x22"),
			Account: &research.Substate_Account{
				Nonce:   proto.Uint64(0x23),
				Balance: U("0x23"),
			},
		},
	)

	x := proto.Clone(s).(*research.Substate)
	x.OutputAlloc.Alloc = []*research.Substate_AllocEntry{
		s.OutputAlloc.Alloc[0],
	}

	y := proto.Clone(s).(*research.Substate)
	y.OutputAlloc.Alloc = []*research.Substate_AllocEntry{
		s.OutputAlloc.Alloc[1],
	}

	var eqAlloc bool

	eqAlloc, _ = EquivalentAlloc(x, y)
	if eqAlloc {
		t.Errorf("eqAlloc must be false for wrong account deletion")
	}

	eqAlloc, _ = EquivalentAlloc(y, x)
	if eqAlloc {
		t.Errorf("eqAlloc must be false for wrong account deletion")
	}
}

func TestEquivalentAllocAccountCreation1(t *testing.T) {
	x := newTestSubstate()
	y := proto.Clone(x).(*research.Substate)

	x.OutputAlloc.Alloc = append(x.OutputAlloc.Alloc, nil)
	x.OutputAlloc.Alloc[1] = &research.Substate_AllocEntry{
		Address: A("0x22"),
		Account: &research.Substate_Account{
			Nonce:   proto.Uint64(0x22),
			Balance: U("0x22"),
		},
	}

	var eqAlloc bool

	eqAlloc, _ = EquivalentAlloc(x, y)
	if eqAlloc {
		t.Errorf("eqAlloc must be false for missing account creation")
	}

	eqAlloc, _ = EquivalentAlloc(y, x)
	if eqAlloc {
		t.Errorf("eqAlloc must be false for missing account creation")
	}
}

func TestEquivalentAllocAccountCreation2(t *testing.T) {
	x := newTestSubstate()
	y := proto.Clone(x).(*research.Substate)

	y.OutputAlloc.Alloc = append(x.OutputAlloc.Alloc, nil)
	y.OutputAlloc.Alloc[1] = &research.Substate_AllocEntry{
		Address: A("0x22"),
		Account: &research.Substate_Account{
			Nonce:   proto.Uint64(0x22),
			Balance: U("0x22"),
		},
	}

	var eqAlloc bool

	eqAlloc, _ = EquivalentAlloc(x, y)
	if eqAlloc {
		t.Errorf("eqAlloc must be false for wrong account creation")
	}

	eqAlloc, _ = EquivalentAlloc(y, x)
	if eqAlloc {
		t.Errorf("eqAlloc must be false for wrong account creation")
	}
}

/*
TestEquivalentAllocAccount_2117d3e7be_6042097_0 tests the corner case from the
transaction at block 6042097, tx 0 which makes func EquivalentAlloc of commit
2117d3e7be to raise "index out of range" error.
1. Same OutputAlloc except some storage keys,
2. x.InputAlloc is longer than y.InputAlloc,
3. Addresses from x.InputAlloc are smaller than or the largest address from
y.InputAlloc.
*/
func TestEquivalentAllocAccount_2117d3e7be_6042097_0(t *testing.T) {
	x := newTestSubstate()
	x.InputAlloc.Alloc = append(
		x.InputAlloc.Alloc,
		&research.Substate_AllocEntry{
			Address: A("0x33"),
			Account: &research.Substate_Account{
				Nonce:   proto.Uint64(0),
				Balance: U("0x00"),
			},
		},
		&research.Substate_AllocEntry{
			Address: A("0x44"),
			Account: &research.Substate_Account{
				Nonce:   proto.Uint64(0),
				Balance: U("0x00"),
				Storage: []*research.Substate_Account_StorageEntry{
					{Key: H("0x00"), Value: H("0x00")},
				},
			},
		},
	)
	x.OutputAlloc.Alloc = append(
		x.OutputAlloc.Alloc,
		&research.Substate_AllocEntry{
			Address: A("0x44"),
			Account: &research.Substate_Account{
				Nonce:   proto.Uint64(0),
				Balance: U("0x00"),
				Storage: []*research.Substate_Account_StorageEntry{
					{Key: H("0x00"), Value: H("0x00")},
				},
			},
		},
	)

	y := newTestSubstate()
	y.InputAlloc.Alloc = append(
		y.InputAlloc.Alloc,
		&research.Substate_AllocEntry{
			Address: A("0x44"),
			Account: &research.Substate_Account{
				Nonce:   proto.Uint64(0),
				Balance: U("0x00"),
				Storage: []*research.Substate_Account_StorageEntry{
					{Key: H("0x11"), Value: H("0x00")},
				},
			},
		},
	)
	y.OutputAlloc.Alloc = append(
		y.OutputAlloc.Alloc,
		&research.Substate_AllocEntry{
			Address: A("0x44"),
			Account: &research.Substate_Account{
				Nonce:   proto.Uint64(0),
				Balance: U("0x00"),
				Storage: []*research.Substate_Account_StorageEntry{
					{Key: H("0x11"), Value: H("0x00")},
				},
			},
		},
	)

	var eqAlloc bool

	eqAlloc, _ = EquivalentAlloc(x, y)
	if eqAlloc {
		t.Errorf("eqAlloc must be false for wrong account deletion")
	}

	eqAlloc, _ = EquivalentAlloc(y, x)
	if eqAlloc {
		t.Errorf("eqAlloc must be false for wrong account deletion")
	}
}
