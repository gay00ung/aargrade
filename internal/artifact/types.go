package artifact

const InspectorVersion = "aar-v1+jvm-classfile-v1"

type Entry struct {
	Name   string `json:"name"`
	Size   uint64 `json:"size"`
	SHA256 string `json:"sha256"`
}

type Member struct {
	Kind       string   `json:"kind"`
	Name       string   `json:"name"`
	Descriptor string   `json:"descriptor"`
	Signature  string   `json:"signature,omitempty"`
	Access     uint16   `json:"access"`
	Constant   string   `json:"constant,omitempty"`
	Exceptions []string `json:"exceptions,omitempty"`
}

func (m Member) Key() string {
	return m.Kind + " " + m.Name + m.Descriptor
}

type Class struct {
	Name                string   `json:"name"`
	Super               string   `json:"super,omitempty"`
	Interfaces          []string `json:"interfaces,omitempty"`
	PermittedSubclasses []string `json:"permittedSubclasses,omitempty"`
	Signature           string   `json:"signature,omitempty"`
	Access              uint16   `json:"access"`
	Members             []Member `json:"members,omitempty"`
	KotlinMetadata      bool     `json:"kotlinMetadata,omitempty"`
}

type RuleIssue struct {
	ID       string `json:"id"`
	Severity string `json:"severity"`
	Path     string `json:"path"`
	Line     int    `json:"line"`
	Rule     string `json:"rule"`
	Message  string `json:"message"`
}

type NativeLibrary struct {
	ABI    string `json:"abi"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type Snapshot struct {
	Inspector      string            `json:"inspector"`
	Path           string            `json:"path"`
	SHA256         string            `json:"sha256"`
	Size           int64             `json:"size"`
	Entries        []Entry           `json:"entries"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	Classes        []Class           `json:"classes,omitempty"`
	KotlinMetadata bool              `json:"kotlinMetadata"`
	RuleFiles      []string          `json:"ruleFiles,omitempty"`
	RuleIssues     []RuleIssue       `json:"ruleIssues,omitempty"`
	Native         []NativeLibrary   `json:"nativeLibraries,omitempty"`
	HasManifest    bool              `json:"hasManifest"`
	HasClassesJar  bool              `json:"hasClassesJar"`
}

type ABIComparison struct {
	Engine              string   `json:"engine"`
	Compatible          bool     `json:"compatible"`
	BaselineClassCount  int      `json:"baselineClassCount"`
	CandidateClassCount int      `json:"candidateClassCount"`
	RemovedClasses      []string `json:"removedClasses,omitempty"`
	RemovedMembers      []string `json:"removedMembers,omitempty"`
	IncompatibleChanges []string `json:"incompatibleChanges,omitempty"`
	Warnings            []string `json:"warnings,omitempty"`
}
