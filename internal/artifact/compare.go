package artifact

import (
	"fmt"
	"slices"
	"sort"
)

func CompareABI(baseline, candidate Snapshot) ABIComparison {
	result := ABIComparison{
		Engine:              "jvm-classfile-v1",
		Compatible:          true,
		BaselineClassCount:  len(baseline.Classes),
		CandidateClassCount: len(candidate.Classes),
	}
	baselineClasses := classMap(baseline.Classes)
	candidateClasses := classMap(candidate.Classes)
	for name, before := range baselineClasses {
		after, ok := candidateClasses[name]
		if !ok {
			result.RemovedClasses = append(result.RemovedClasses, name)
			continue
		}
		compareClass(&result, before, after)
	}
	sort.Strings(result.RemovedClasses)
	sort.Strings(result.RemovedMembers)
	sort.Strings(result.IncompatibleChanges)
	sort.Strings(result.Warnings)
	result.Compatible = len(result.RemovedClasses) == 0 && len(result.RemovedMembers) == 0 && len(result.IncompatibleChanges) == 0
	return result
}

func compareClass(result *ABIComparison, before, after Class) {
	if accessRank(after.Access) < accessRank(before.Access) {
		result.IncompatibleChanges = append(result.IncompatibleChanges, fmt.Sprintf("%s class access was narrowed", before.Name))
	}
	if before.Super != after.Super {
		result.IncompatibleChanges = append(result.IncompatibleChanges, fmt.Sprintf("%s superclass changed: %s -> %s", before.Name, before.Super, after.Super))
	}
	afterInterfaces := stringSet(after.Interfaces)
	for _, name := range before.Interfaces {
		if !afterInterfaces[name] {
			result.IncompatibleChanges = append(result.IncompatibleChanges, fmt.Sprintf("%s no longer implements %s", before.Name, name))
		}
	}
	if before.Access&accInterface != after.Access&accInterface {
		result.IncompatibleChanges = append(result.IncompatibleChanges, fmt.Sprintf("%s changed between class and interface", before.Name))
	}
	if before.Access&accFinal == 0 && after.Access&accFinal != 0 {
		result.IncompatibleChanges = append(result.IncompatibleChanges, fmt.Sprintf("%s became final", before.Name))
	}
	if before.Access&accAbstract == 0 && after.Access&accAbstract != 0 {
		result.IncompatibleChanges = append(result.IncompatibleChanges, fmt.Sprintf("%s became abstract", before.Name))
	}
	if len(before.PermittedSubclasses) == 0 && len(after.PermittedSubclasses) > 0 && before.Access&accFinal == 0 {
		result.IncompatibleChanges = append(result.IncompatibleChanges, fmt.Sprintf("%s became sealed", before.Name))
	}
	afterPermitted := stringSet(after.PermittedSubclasses)
	if len(after.PermittedSubclasses) > 0 {
		for _, name := range before.PermittedSubclasses {
			if !afterPermitted[name] {
				result.IncompatibleChanges = append(result.IncompatibleChanges, fmt.Sprintf("%s no longer permits subclass %s", before.Name, name))
			}
		}
	}
	if before.Signature != after.Signature {
		result.Warnings = append(result.Warnings, fmt.Sprintf("%s generic signature changed", before.Name))
	}

	afterMembers := memberMap(after.Members)
	for _, member := range before.Members {
		candidate, ok := afterMembers[member.Key()]
		if !ok {
			result.RemovedMembers = append(result.RemovedMembers, before.Name+"#"+member.Key())
			continue
		}
		if accessRank(candidate.Access) < accessRank(member.Access) {
			result.IncompatibleChanges = append(result.IncompatibleChanges, before.Name+"#"+member.Key()+" access was narrowed")
		}
		for _, flag := range []struct {
			mask uint16
			name string
		}{{accStatic, "static"}, {accAbstract, "abstract"}} {
			if member.Access&flag.mask != candidate.Access&flag.mask {
				result.IncompatibleChanges = append(result.IncompatibleChanges, fmt.Sprintf("%s#%s changed %s semantics", before.Name, member.Key(), flag.name))
			}
		}
		if member.Access&accFinal == 0 && candidate.Access&accFinal != 0 &&
			(member.Kind == "field" || candidate.Access&accStatic == 0) {
			result.IncompatibleChanges = append(result.IncompatibleChanges, before.Name+"#"+member.Key()+" became final")
		}
		if member.Signature != candidate.Signature {
			result.Warnings = append(result.Warnings, before.Name+"#"+member.Key()+" generic signature changed")
		}
		if member.Constant != candidate.Constant {
			result.Warnings = append(result.Warnings, before.Name+"#"+member.Key()+" constant value changed")
		}
		if !slices.Equal(member.Exceptions, candidate.Exceptions) {
			result.Warnings = append(result.Warnings, before.Name+"#"+member.Key()+" declared exceptions changed")
		}
	}
}

func classMap(classes []Class) map[string]Class {
	result := make(map[string]Class, len(classes))
	for _, class := range classes {
		result[class.Name] = class
	}
	return result
}

func memberMap(members []Member) map[string]Member {
	result := make(map[string]Member, len(members))
	for _, member := range members {
		result[member.Key()] = member
	}
	return result
}

func stringSet(values []string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}

func accessRank(access uint16) int {
	if access&accPublic != 0 {
		return 2
	}
	if access&accProtected != 0 {
		return 1
	}
	return 0
}
