package gooo_workspace

//gooo:generated:start id="gooo://service-impact/decision/v1" kind="entity"
type GoooRecord8caadab4cd27247527aeb5d57230dc24cb20b32dd532144849f72efd0ed67153 struct {
	GoooFielddbb72b1dd94b1481e9a1dbe9ea75cb83fd7073af74987154593d2dad6adb649d bool   `json:"before_match"`
	GoooFielda820b495c790dce4367677d8cf1a3fd0e7cff0db75cf63a37389d5c91923c219 bool   `json:"after_match"`
	GoooField9e32e23e3055b6c63e2d262d3924843e69f7c0c733913e3c8ee8d8f843929c4d string `json:"change"`
}

//gooo:generated:end id="gooo://service-impact/decision/v1" kind="entity"
//gooo:generated:start id="gooo://service-impact/compare" kind="activity"
func GoooPackage1ActivityCompareServiceGrant(input0 string, input1 string, input2 string, input3 string) GoooRecord8caadab4cd27247527aeb5d57230dc24cb20b32dd532144849f72efd0ed67153 {

	var before = GoooPackage0ActivityMatchServicePattern(input0, input2, input3)
	var after = GoooPackage0ActivityMatchServicePattern(input1, input2, input3)
	var change = "UNCHANGED_DENY"
	if before {
		if after {
			change = "UNCHANGED_ALLOW"
		} else {
			change = "REVOKED"
		}
	} else {
		if after {
			change = "WIDENED"
		}
	}
	return GoooRecord8caadab4cd27247527aeb5d57230dc24cb20b32dd532144849f72efd0ed67153{GoooFielddbb72b1dd94b1481e9a1dbe9ea75cb83fd7073af74987154593d2dad6adb649d: before, GoooFielda820b495c790dce4367677d8cf1a3fd0e7cff0db75cf63a37389d5c91923c219: after, GoooField9e32e23e3055b6c63e2d262d3924843e69f7c0c733913e3c8ee8d8f843929c4d: change}

}

//gooo:generated:end id="gooo://service-impact/compare" kind="activity"
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
