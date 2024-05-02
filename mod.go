package gosym

import (
	"encoding/binary"
	"fmt"
)

func (f *Func) TablePCFileSize() int {
	return f.tableSize(f.OffPCFile)
}

func (f *Func) TablePCSPSize() int {
	return f.tableSize(f.OffPCSP)
}

func (f *Func) TablePCLnSize() int {
	return f.tableSize(f.OffPCLn)
}

func (f *Func) FixedHeaderSize() int {
	return int(f.LineTable.ptrsize + 8* /* uint32 size */ 4)
}

func pcdataSuffix(n int) string {
	switch n {
	case 0:
		return "-unsafepoint"
	case 1:
		return "-stackmap"
	case 2:
		return "-inltree"
	}
	return ""
}

func (f *Func) PCDataSize() map[string]int {
	ret := make(map[string]int)
	for i := 0; i < f.NumPCData; i++ {
		name := fmt.Sprintf("pcdata-%d%s", i, pcdataSuffix(i))
		ret[name] = f.tableSizePCData(i) + /* offset pointer */ 4
	}
	return ret
}

// tab is 0-based table number.
func (f *Func) tableSizePCData(tab int) int {
	if tab >= f.NumPCData || tab < 0 {
		panic(fmt.Errorf("bogus tab %d; NumPCData=%v", tab, f.NumPCData))
	}
	fs := funcData{f.LineTable, f.funcDataBytes}
	tableOff := fs.tableOff(uint32(tab))
	if tableOff == 0 {
		return 0
	}
	return f.tableSize(tableOff)
}

func (f *Func) tableSize(off uint32) int {
	sumSize := 0
	f.ForeachTableEntry(off, func(val int64, valBytes int, pc uint64, pcBytes int) {
		sumSize += valBytes + pcBytes
	})
	return sumSize
}

func (f *Func) ForeachTableEntry(off uint32, fn func(val int64, valBytes int, pc uint64, pcBytes int)) {
	if off == 0 {
		return
	}
	data := f.LineTable.pctab[off:]
	pc := f.Entry
	val := int64(-1)

	for len(data) > 0 && pc < f.End {
		vald, valBytes := binary.Varint(data)
		if valBytes <= 0 {
			panic("bogus")
		}
		val += vald
		data = data[valBytes:]

		pcd, pcBytes := binary.Uvarint(data)
		if pcBytes <= 0 {
			panic("bogus")
		}

		data = data[pcBytes:]
		pcd *= uint64(f.LineTable.quantum)
		pc += pcd

		fn(val, valBytes, pc, pcBytes)
	}
}

/*
From src/cmd/link/internal/ld/pcln.go.writeFuncs() and src/runtime/runtime2.go._func:

	uint32 entryOff;	// offset of func entry PC from textStart
	int32 nameOff;		// name (offset to C string)
	int32 args;			// size of arguments passed to function
	uint32 deferreturn;	// size of function frame, including saved caller PC
	uint32 pcsp;		// pcsp table (offset to pcvalue table)
	uint32 pcfile;		// pcfile table (offset to pcvalue table)
	uint32 pcln;		// pcln table (offset to pcvalue table)
	uint32 npcdata;		// number of entries in pcdata list
	uint32 cuoffset; // 1.16+
	int32 startline; // 1.20+
	uint8 funcID;
	uint8 flag; // 1.17+
	// 1 byte padding or 2 bytes <= 1.16
	uint8 nfuncdata;	// number of entries in funcdata list

*/

func (f funcData) pcsp() uint32   { return f.field(4) }
func (f funcData) numPCData() int { return int(f.field(7)) }

func (f funcData) numFuncData() int {
	return int(f.field(f.nfuncdataFieldNum()) & 255)
}

func (f funcData) nfuncdataFieldNum() uint32 {
	if f.t.version < ver116 {
		return 8
	}
	if f.t.version < ver120 {
		return 9
	}
	return 10
}

func (f funcData) tableOff(tab uint32) uint32 {
	return f.field(f.nfuncdataFieldNum() + 1 + tab)
}
