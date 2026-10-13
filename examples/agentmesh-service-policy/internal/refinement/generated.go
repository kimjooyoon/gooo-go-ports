package gooo_workspace

//gooo:generated:start id="gooo://service-refinement/decision/v1" kind="entity"
type GoooRecord702bf03e1b9b659de2cca8f66c90d35af72f6f992a24d89cf90ce7c269907367 struct {
	GoooFieldaf6991124a762ff89da9eae9eccd222db33bb2ddf56b92993d7bd2cf5c9ad354 string `json:"status"`
	GoooField24698bac43894a5573e5dadbf8c2e52cf44a1615484c51a4d0e675600a8d0454 string `json:"rule_id"`
	GoooFielda09c15e873f28daf8cbd9c93c478c1480486831d7c55656c7d5491fbf2c52711 bool   `json:"supported_subset"`
	GoooFieldbba889b4d1fcabf4151fd8dad7f727eb777e045df9c39fff7dc72f4401ec003d bool   `json:"has_witness"`
	GoooField02e0207ce9dd381ba57b20243ee685352c92be98dc5d7fe8d6e33525e5a3dfbd string `json:"witness_type"`
	GoooFieldd602c0b83fa88a27ce104dff11a826ceeeb694a927fe062f415d97ce859b53d6 string `json:"witness_name"`
	GoooFieldabdccd9e95eef4d357adccc51e4976e882b3d7a22d6b8701951a951f3eb4c4cc bool   `json:"before_match"`
	GoooField25664eeb9a2c3098e71e67b9fbfd995fdeecbd8e091db678ed6d4b66defab7ef bool   `json:"after_match"`
}

//gooo:generated:end id="gooo://service-refinement/decision/v1" kind="entity"
//gooo:generated:start id="gooo://service-refinement/analyze" kind="activity"
func GoooPackage1ActivityAnalyzeServiceGrant(input0 string, input1 string) GoooRecord702bf03e1b9b659de2cca8f66c90d35af72f6f992a24d89cf90ce7c269907367 {

	var rule = ""
	if input1 == "" {
		rule = "CANDIDATE_EMPTY"
	} else {
		if input0 == input1 {
			rule = "IDENTICAL_PATTERNS"
		} else {
			if input0 == "*" {
				rule = "OLD_GLOBAL"
			}
		}
	}
	var oldType = ""
	var candidateType = ""
	var oldBase = ""
	var candidateBase = ""
	var oldKind = "NONE"
	var candidateKind = "NONE"
	var oldSplit int64 = -1
	var candidateSplit int64 = -1
	var offset int64 = 0
	for offset = 0; offset < 128; offset = offset + 1 {
		var end int64 = offset + 3
		if end <= int64(len(input0)) {
			if input0[offset:end] == "://" {
				if oldSplit == -1 {
					oldSplit = offset
				}
			}
		}
		if end <= int64(len(input1)) {
			if input1[offset:end] == "://" {
				if candidateSplit == -1 {
					candidateSplit = offset
				}
			}
		}
	}
	if oldSplit > 0 {
		oldType = input0[0:oldSplit]
		var start int64 = oldSplit + 3
		oldBase = input0[start:int64(len(input0))]
		oldKind = "EXACT"
		if oldBase == "*" {
			oldKind = "TYPED_ALL"
		} else {
			var n = int64(len(oldBase))
			if n >= 2 {
				if oldBase[0:2] == "*." {
					oldKind = "SUFFIX"
					oldBase = oldBase[2:n]
				} else {
					var tail = n - 2
					if oldBase[tail:n] == ".*" {
						oldKind = "PREFIX"
						oldBase = oldBase[0:tail]
					}
				}
			}
		}
	}
	if candidateSplit > 0 {
		candidateType = input1[0:candidateSplit]
		var start int64 = candidateSplit + 3
		candidateBase = input1[start:int64(len(input1))]
		candidateKind = "EXACT"
		if candidateBase == "*" {
			candidateKind = "TYPED_ALL"
		} else {
			var n = int64(len(candidateBase))
			if n >= 2 {
				if candidateBase[0:2] == "*." {
					candidateKind = "SUFFIX"
					candidateBase = candidateBase[2:n]
				} else {
					var tail = n - 2
					if candidateBase[tail:n] == ".*" {
						candidateKind = "PREFIX"
						candidateBase = candidateBase[0:tail]
					}
				}
			}
		}
	}
	if rule == "" {
		if oldKind == "TYPED_ALL" && candidateType == oldType {
			rule = "OLD_TYPED_ALL"
		}
	}
	if rule == "" {
		if candidateKind == "EXACT" {
			if GoooPackage0ActivityMatchServicePattern(input0, candidateType, candidateBase) {
				rule = "EXACT_CANDIDATE_ACCEPTED_BY_OLD"
			}
		}
	}
	if rule == "" && oldType == candidateType {
		if oldKind == "PREFIX" && candidateKind == "PREFIX" {
			if oldBase == candidateBase {
				rule = "PREFIX_HIERARCHY"
			} else {
				var end = int64(len(oldBase)) + 1
				if int64(len(candidateBase)) >= end {
					if candidateBase[0:end] == oldBase+"." {
						rule = "PREFIX_HIERARCHY"
					}
				}
			}
		}
		if oldKind == "SUFFIX" && candidateKind == "SUFFIX" {
			if oldBase == candidateBase {
				rule = "SUFFIX_HIERARCHY"
			} else {
				var n = int64(len(candidateBase))
				var size = int64(len(oldBase)) + 1
				if n >= size {
					var start = n - size
					if candidateBase[start:n] == "."+oldBase {
						rule = "SUFFIX_HIERARCHY"
					}
				}
			}
		}
	}
	var status = "UNPROVEN"
	var supported = false
	if rule != "" {
		status = "SUPPORTED_SUBSET"
		supported = true
	}
	var witnessed = false
	var witnessType = ""
	var witnessName = ""
	var witnessBefore = false
	var witnessAfter = false
	if rule == "" {
		var firstType = candidateType
		var secondType = candidateType
		var firstName = ""
		var secondName = ""
		if input1 == "*" {
			firstType = "a"
			secondType = "b"
			firstName = "x"
			secondName = "x"
		}
		if candidateKind == "TYPED_ALL" {
			firstName = "x"
			secondName = "y"
		}
		if candidateKind == "EXACT" {
			firstName = candidateBase
		}
		if candidateKind == "PREFIX" {
			firstName = candidateBase + ".x"
			secondName = candidateBase + ".y"
		}
		if candidateKind == "SUFFIX" {
			firstName = "x." + candidateBase
			secondName = "y." + candidateBase
		}
		if firstName != "" {
			var before = GoooPackage0ActivityMatchServicePattern(input0, firstType, firstName)
			var after = GoooPackage0ActivityMatchServicePattern(input1, firstType, firstName)
			if after && !before {
				witnessed = true
				witnessType = firstType
				witnessName = firstName
				witnessBefore = before
				witnessAfter = after
			}
		}
		if !witnessed && secondName != "" {
			var before = GoooPackage0ActivityMatchServicePattern(input0, secondType, secondName)
			var after = GoooPackage0ActivityMatchServicePattern(input1, secondType, secondName)
			if after && !before {
				witnessed = true
				witnessType = secondType
				witnessName = secondName
				witnessBefore = before
				witnessAfter = after
			}
		}
		if witnessed {
			status = "WIDENING_WITNESS"
			rule = "CHECKED_CANDIDATE_REQUEST"
		}
	}
	return GoooRecord702bf03e1b9b659de2cca8f66c90d35af72f6f992a24d89cf90ce7c269907367{GoooFieldaf6991124a762ff89da9eae9eccd222db33bb2ddf56b92993d7bd2cf5c9ad354: status, GoooField24698bac43894a5573e5dadbf8c2e52cf44a1615484c51a4d0e675600a8d0454: rule, GoooFielda09c15e873f28daf8cbd9c93c478c1480486831d7c55656c7d5491fbf2c52711: supported, GoooFieldbba889b4d1fcabf4151fd8dad7f727eb777e045df9c39fff7dc72f4401ec003d: witnessed, GoooField02e0207ce9dd381ba57b20243ee685352c92be98dc5d7fe8d6e33525e5a3dfbd: witnessType, GoooFieldd602c0b83fa88a27ce104dff11a826ceeeb694a927fe062f415d97ce859b53d6: witnessName, GoooFieldabdccd9e95eef4d357adccc51e4976e882b3d7a22d6b8701951a951f3eb4c4cc: witnessBefore, GoooField25664eeb9a2c3098e71e67b9fbfd995fdeecbd8e091db678ed6d4b66defab7ef: witnessAfter}

}

//gooo:generated:end id="gooo://service-refinement/analyze" kind="activity"
//gooo:generated:start id="gooo://agentmesh-prototype/match-service-pattern" kind="activity"
func GoooPackage0ActivityMatchServicePattern(input0 string, input1 string, input2 string) bool {

	if input0 == "" || input1 == "" || input2 == "" {
		return false
	}
	if input0 == "*" {
		return true
	}
	var split int64 = -1
	var offset int64 = 0
	for offset = 0; offset < 128; offset = offset + 1 {
		var end int64 = offset + 3
		if end <= int64(len(input0)) {
			if input0[offset:end] == "://" {
				if split == -1 {
					split = offset
				}
			}
		}
	}
	if split < 0 {
		return false
	}
	var nameStart int64 = split + 3
	var patternType = input0[0:split]
	var patternName = input0[nameStart:int64(len(input0))]
	if patternType != input1 || patternName == "" {
		return false
	}
	if patternName == "*" {
		return true
	}
	var patternLength = int64(len(patternName))
	var requestLength = int64(len(input2))
	if patternLength >= 2 {
		if patternName[0:2] == "*." {
			var suffix = patternName[1:patternLength]
			var suffixLength = int64(len(suffix))
			if requestLength < suffixLength {
				return false
			}
			var suffixStart = requestLength - suffixLength
			return input2[suffixStart:requestLength] == suffix
		}
		var tailStart = patternLength - 2
		if patternName[tailStart:patternLength] == ".*" {
			var prefixEnd = patternLength - 1
			var prefix = patternName[0:prefixEnd]
			if requestLength < prefixEnd {
				return false
			}
			return input2[0:prefixEnd] == prefix
		}
	}
	return input2 == patternName

}

//gooo:generated:end id="gooo://agentmesh-prototype/match-service-pattern" kind="activity"
