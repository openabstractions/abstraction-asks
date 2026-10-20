package asks

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// ProgramShareDivisor sets one program's share of each admission partition
// (ASK-P5): an eighth of the legacy and typed partition's admissions and
// retained records, and an eighth of the ordinary partition's admissions and
// bytes. One program that asks without end reads exhausted at its share, and
// every other program keeps the rest; the partitions' own bounds (ASK-P2,
// ASK-P4) still hold for all of them together. Notifications keep their own
// per-program rate (ASK-N6).
const ProgramShareDivisor = 8

// programApplicationShare and programOrdinaryShare are the resulting
// per-program counts; programOrdinaryBytes the ordinary byte share.
const (
	programApplicationShare = MaxApplicationAdmissions / ProgramShareDivisor
	programOrdinaryShare    = MaxOrdinaryAdmissions / ProgramShareDivisor
	programOrdinaryBytes    = MaxOrdinaryBytes / ProgramShareDivisor
)

// programHash is the fingerprint a program's share is counted under: the
// SHA-256 of its recorded spelling. An admission whose record is gone, a
// retired question's or an acknowledged choice's, keeps this fingerprint
// (program_hash) and stays in the share of the program that asked.
func programHash(program string) string {
	sum := sha256.Sum256([]byte(program))
	return hex.EncodeToString(sum[:])
}

// shareKey is the share a program fingerprint counts against, or the
// admitting scope's when none is recorded.
func shareKey(hash, scope string) string {
	if hash != "" {
		return "program:" + hash
	}
	return "scope:" + scope
}

// recordProgram is the program a legacy or typed record counts against: the
// program a runtime first-use question is about, the program a typed request
// is for, or the bound asker; "" when it records none.
func recordProgram(r *Record) string {
	switch {
	case r == nil:
		return ""
	case r.Permission != nil && len(r.Permission.Targets) > 0:
		return r.Permission.Targets[0].Program
	case r.TypedPermission != nil:
		return r.TypedPermission.Effect.Subject.Program
	}
	return r.AskerProgram
}

// recordProgramHash is programHash of recordProgram, or "" for a record
// without a program.
func recordProgramHash(r *Record) string {
	if program := recordProgram(r); program != "" {
		return programHash(program)
	}
	return ""
}

// programShare is the share of program admitted in scope; a request that
// records no program counts against its scope.
func programShare(program, scope string) string {
	if program == "" {
		return shareKey("", scope)
	}
	return shareKey(programHash(program), scope)
}

// recordShare is the share a record admitted in scope counts against.
func recordShare(r *Record, scope string) string {
	return programShare(recordProgram(r), scope)
}

// applicationShare counts the retained records and admissions of one share
// in the legacy and typed partition. An admission counts against the
// program fingerprint it kept, else its record's program, else its scope.
func (f *book) applicationShare(share string) (records, admissions int) {
	owners := make(map[string]string, len(f.Asks))
	nativeShares := make(map[string]string, len(f.Asks))
	for _, ad := range f.Admissions {
		if _, ok := owners[ad.ID]; !ok {
			owners[ad.ID] = ad.Scope
			if ad.NativeOwner != nil {
				nativeShares[ad.ID] = programShare(ad.NativeOwner.Subject.Program, ad.Scope)
			}
		}
	}
	byID := make(map[string]*Record, len(f.Asks))
	for _, r := range f.Asks {
		byID[r.ID] = r
		recordOwner := recordShare(r, owners[r.ID])
		if canonical, ok := nativeShares[r.ID]; ok {
			recordOwner = canonical
		}
		if recordOwner == share {
			records++
		}
	}
	for _, ad := range f.Admissions {
		hash := ad.ProgramHash
		if hash == "" {
			hash = ad.NativeOwner.programFingerprint(ad.Scope, byID[ad.ID])
		}
		if shareKey(hash, ad.Scope) == share {
			admissions++
		}
	}
	return records, admissions
}

// fillProgramHashes gives each admission without a program fingerprint its
// retained record's, if that record has a program.
func (f *book) fillProgramHashes() {
	byID := make(map[string]*Record, len(f.Asks))
	for _, r := range f.Asks {
		byID[r.ID] = r
	}
	for i := range f.Admissions {
		if r, ok := byID[f.Admissions[i].ID]; ok && f.Admissions[i].ProgramHash == "" {
			f.Admissions[i].ProgramHash = f.Admissions[i].NativeOwner.programFingerprint(f.Admissions[i].Scope, r)
		}
	}
}

// applicationShareFull reports whether one more admission, and with newRecord
// one more record, would take the share past its bound.
func (f *book) applicationShareFull(share string, newRecord bool) bool {
	records, admissions := f.applicationShare(share)
	return admissions >= programApplicationShare || newRecord && records >= programApplicationShare
}

// choiceShare is the share an ordinary admission counts against: its bound
// requester, the requester fingerprint its acknowledgement kept, or its
// scope.
func choiceShare(a choiceAdmission) string {
	if a.NativeOwner != nil {
		return programShare(a.NativeOwner.Subject.Program, a.Scope)
	}
	if a.AskerProgram != "" {
		return programShare(a.AskerProgram, a.Scope)
	}
	return shareKey(a.ProgramHash, a.Scope)
}

// ordinaryShareFull reports whether admitting next would take its share past
// its ordinary bound of admissions or bytes.
func (f *book) ordinaryShareFull(next choiceAdmission) bool {
	share := choiceShare(next)
	mine := []choiceAdmission{next}
	for _, a := range f.Choices {
		if choiceShare(a) == share || next.NativeOwner != nil && a.Scope == next.NativeOwner.legacy() {
			mine = append(mine, a)
		}
	}
	if len(mine) > programOrdinaryShare {
		return true
	}
	raw, err := json.Marshal(mine)
	return err != nil || len(raw) > programOrdinaryBytes
}
