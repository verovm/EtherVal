package parser

import (
	"bytes"
	"sync"

	lru "github.com/hashicorp/golang-lru"

	"github.com/seonghojj/substrate_parser/ast"
)

var (
	safeParseLock     = sync.Mutex{}          // Mutex to make Lexer thread-safe
	safeParseCache, _ = lru.NewARC(16 * 1024) // Thread-safe ARC cache
)

// SafeParse is thread-safe function to parse gigahorse substrate,
// optimized with memoization technique.
func SafeParse(substrate []byte) *ast.SubstrateNode {
	var (
		root       *ast.SubstrateNode
		key, value interface{}
		ok         bool
	)

	key = string(substrate)
	// Try load AST from global thread-safe map first
	if value, ok = safeParseCache.Get(key); !ok {
		// No memorized value - compute AST and store it
		// Lock global mutex to call Lexer with global variables safely
		safeParseLock.Lock()
		defer safeParseLock.Unlock()

		// Check once more if any thread parsed this substrate while waiting
		if value, ok = safeParseCache.Get(key); !ok {
			// No thread parsed it before - compute AST
			l := NewLexer(bytes.NewReader(substrate), false)
			if ItemParse(l) != 0 {
				//panic("Error in parsing")
				return nil
			}

			// Store computed AST to global thread-safe map
			value = l.Root
			safeParseCache.Add(key, value)
		}
	}

	// Memorized value exists - cast it to *ast.SubstrateNode type
	root, ok = value.(*ast.SubstrateNode)
	if !ok {
		panic("Failed to cast memorized value to *ast.SubstrateNode type")
	}

	tmp := bytes.Split(substrate, []byte("\n"))
	root.Date = string(bytes.ReplaceAll(tmp[1][3:13], []byte("."), nil))

	return root
}
