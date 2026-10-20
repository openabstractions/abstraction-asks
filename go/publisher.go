package asks

import (
	"github.com/openabstractions/abstraction-asks/go/typedpermission"

	identity "github.com/openabstractions/abstraction-identity"
)

// maxPublisher bounds each recorded publisher string in UTF-8 bytes: the
// subject, the program description and the program's original file name.
const maxPublisher = typedpermission.MaxPublisher

// Publisher is the requesting program's code signer as the admitting service
// verified it at admission (ASK-A6). Subject names the signer; Trusted is the
// platform trust policy's verdict. Description and OriginalName are the
// program's signed version resource (FileDescription and OriginalFilename),
// present only with a trusted signature. The zero value records no evidence.
type Publisher struct {
	Subject      string
	Trusted      bool
	Description  string
	OriginalName string
}

// CodePublisher is the publisher evidence of a platform code verification.
// A string that is too long, invalid UTF-8, or carries a control or format
// character (which could make a prompt name another signer or program) is not
// recorded. The version resource is recorded only for a trusted signature.
func CodePublisher(code identity.Code) Publisher {
	return Publisher{Subject: code.Subject, Trusted: code.Trusted,
		Description: code.Description, OriginalName: code.OriginalName}.recorded()
}

// recorded is the publisher as the Book keeps it: each string bounded, and the
// version resource only beside a trusted signature.
func (p Publisher) recorded() Publisher {
	return Publisher(typedpermission.RecordPublisher(typedpermission.Publisher(p)))
}
func publisherSubject(subject string) string { return typedpermission.PublisherSubject(subject) }
func validRecordedPublisher(r *Record) bool {
	p := Publisher{Subject: r.Publisher, Trusted: r.PublisherTrusted, Description: r.ProgramDescription, OriginalName: r.ProgramOriginalName}
	return p.recorded() == p
}
