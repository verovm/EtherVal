package validate

import (
	"bufio"
	"crypto/md5"
	"fmt"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/cmd/substate-cli/rr03/research"
	"github.com/ethereum/go-ethereum/common"

	"github.com/urfave/cli/v2"
)

var (
	AllowlistFlag = &cli.StringFlag{
		Name:  "allowlist",
		Usage: "Path to a file with list of addresses or code hashes allowed for execution",
		Value: "",
	}
)

type Allowlist struct {
	Enabled bool

	AddressSet  map[common.Address]struct{}
	CodeHashSet map[common.Hash]struct{}
	CodeMd5set  map[[md5.Size]byte]struct{}
}

func NewAllowlist() *Allowlist {
	return &Allowlist{
		Enabled: true,

		AddressSet:  make(map[common.Address]struct{}),
		CodeHashSet: make(map[common.Hash]struct{}),
		CodeMd5set:  make(map[[md5.Size]byte]struct{}),
	}
}

func (allowlist *Allowlist) IsDisabled() bool {
	return allowlist == nil || !allowlist.Enabled
}

func (allowlist *Allowlist) Add(elem []byte) {
	switch len(elem) {
	case common.AddressLength:
		allowlist.AddressSet[common.BytesToAddress(elem)] = struct{}{}
	case common.HashLength:
		allowlist.CodeHashSet[common.BytesToHash(elem)] = struct{}{}
	case md5.Size:
		m := [md5.Size]byte{}
		copy(m[:], elem)
		allowlist.CodeMd5set[m] = struct{}{}
	default:
		panic("elem must be either 20-byte address, 32-byte codehash, or 16-byte MD5 hash")
	}
}

func (allowlist *Allowlist) IsAddressAllowed(addr common.Address) bool {
	if allowlist.IsDisabled() {
		return true
	}
	_, has := allowlist.AddressSet[addr]
	return has
}

func (allowlist *Allowlist) IsCodeHashAllowed(codehash common.Hash) bool {
	if allowlist.IsDisabled() {
		return true
	}
	_, has := allowlist.CodeHashSet[codehash]
	return has
}

func (allowlist *Allowlist) IsCodeMd5Allowed(codemd5 [md5.Size]byte) bool {
	if allowlist.IsDisabled() {
		return true
	}
	_, has := allowlist.CodeMd5set[codemd5]
	return has
}

func (allowlist *Allowlist) IsCodeAllowed(code []byte) bool {
	if allowlist.IsDisabled() {
		return true
	}

	// Optimization: if a set is empty, do not compute hash for the set.
	if len(allowlist.CodeHashSet) > 0 && allowlist.IsCodeHashAllowed(research.CodeHash(code)) {
		return true
	}
	if len(allowlist.CodeMd5set) > 0 && allowlist.IsCodeMd5Allowed(md5.Sum(code)) {
		return true
	}

	return false
}

func (allowlist *Allowlist) IsAllowed(addr common.Address, code []byte) bool {
	return allowlist.IsAddressAllowed(addr) || allowlist.IsCodeAllowed(code)
}

func ReadAllowlist(path string) *Allowlist {
	allowlist := NewAllowlist()

	fmt.Printf("reading tx filter %s\n", path)
	filterFile, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer filterFile.Close()

	sc := bufio.NewScanner(filterFile)
	for sc.Scan() {
		line := sc.Text()
		hx := strings.TrimSpace(line)
		if len(hx) > 0 {
			bs := common.FromHex(hx)
			allowlist.Add(bs)
		}
	}

	fmt.Printf("Read %v addresses, %v code hashes, %v md5 hashes from %s\n",
		len(allowlist.AddressSet), len(allowlist.CodeHashSet), len(allowlist.CodeMd5set), path)

	return allowlist
}
