package gooo_workspace

//gooo:generated:start id="gooo://service-impact/decision/v1" kind="entity"
type GoooRecord8caadab4cd27247527aeb5d57230dc24cb20b32dd532144849f72efd0ed67153 struct {
	GoooFielddbb72b1dd94b1481e9a1dbe9ea75cb83fd7073af74987154593d2dad6adb649d bool   `json:"before_match"`
	GoooFielda820b495c790dce4367677d8cf1a3fd0e7cff0db75cf63a37389d5c91923c219 bool   `json:"after_match"`
	GoooField9e32e23e3055b6c63e2d262d3924843e69f7c0c733913e3c8ee8d8f843929c4d string `json:"change"`
}

//gooo:generated:end id="gooo://service-impact/decision/v1" kind="entity"
//gooo:generated:start id="gooo://service-policy/checked-impact/decision/v1" kind="entity"
type GoooRecorda03acd3ba02b82f739d609e25239e3eb36532cb33623ba448b028923604c461e struct {
	GoooFieldb68bddcca03807b9d3b7351580885098ab19653ee50128d38ae96d5bac276650 string `json:"domain_status"`
	GoooFieldc673629c5c89a0b38f8954ebea6f889f2292d582cce3fb1be7194fab70ecfad8 string `json:"invalid_field"`
	GoooFieldcb040b8a819e1010951cd9754fd976778e836fe898076d235b2fb3d3080e02cf bool   `json:"before_match"`
	GoooField9c2e81f8161f0bc825eaa4d4f0f01ecd432178cb7e7db6eee17331a96f8176ac bool   `json:"after_match"`
	GoooField134fcdefde3b0f35a4f54e2c8857d7f11114b89fdac8465fea0b5a08df5295ea string `json:"change"`
}

//gooo:generated:end id="gooo://service-policy/checked-impact/decision/v1" kind="entity"
//gooo:generated:start id="gooo://service-policy/checked-impact/evaluate/v1" kind="activity"
func GoooPackage5ActivityCompareServiceGrantChecked(input0 string, input1 string, input2 string, input3 string) GoooRecorda03acd3ba02b82f739d609e25239e3eb36532cb33623ba448b028923604c461e {

	if !GoooPackage4ActivityValidateServicePattern(input0) {
		return GoooRecorda03acd3ba02b82f739d609e25239e3eb36532cb33623ba448b028923604c461e{GoooFieldb68bddcca03807b9d3b7351580885098ab19653ee50128d38ae96d5bac276650: "INVALID_INPUT", GoooFieldc673629c5c89a0b38f8954ebea6f889f2292d582cce3fb1be7194fab70ecfad8: "before", GoooFieldcb040b8a819e1010951cd9754fd976778e836fe898076d235b2fb3d3080e02cf: false, GoooField9c2e81f8161f0bc825eaa4d4f0f01ecd432178cb7e7db6eee17331a96f8176ac: false, GoooField134fcdefde3b0f35a4f54e2c8857d7f11114b89fdac8465fea0b5a08df5295ea: "INVALID_INPUT"}
	}
	if !GoooPackage4ActivityValidateServicePattern(input1) {
		return GoooRecorda03acd3ba02b82f739d609e25239e3eb36532cb33623ba448b028923604c461e{GoooFieldb68bddcca03807b9d3b7351580885098ab19653ee50128d38ae96d5bac276650: "INVALID_INPUT", GoooFieldc673629c5c89a0b38f8954ebea6f889f2292d582cce3fb1be7194fab70ecfad8: "after", GoooFieldcb040b8a819e1010951cd9754fd976778e836fe898076d235b2fb3d3080e02cf: false, GoooField9c2e81f8161f0bc825eaa4d4f0f01ecd432178cb7e7db6eee17331a96f8176ac: false, GoooField134fcdefde3b0f35a4f54e2c8857d7f11114b89fdac8465fea0b5a08df5295ea: "INVALID_INPUT"}
	}
	if !GoooPackage2ActivityValidateServiceType(input2) {
		return GoooRecorda03acd3ba02b82f739d609e25239e3eb36532cb33623ba448b028923604c461e{GoooFieldb68bddcca03807b9d3b7351580885098ab19653ee50128d38ae96d5bac276650: "INVALID_INPUT", GoooFieldc673629c5c89a0b38f8954ebea6f889f2292d582cce3fb1be7194fab70ecfad8: "req_type", GoooFieldcb040b8a819e1010951cd9754fd976778e836fe898076d235b2fb3d3080e02cf: false, GoooField9c2e81f8161f0bc825eaa4d4f0f01ecd432178cb7e7db6eee17331a96f8176ac: false, GoooField134fcdefde3b0f35a4f54e2c8857d7f11114b89fdac8465fea0b5a08df5295ea: "INVALID_INPUT"}
	}
	if !GoooPackage3ActivityValidateServiceName(input3) {
		return GoooRecorda03acd3ba02b82f739d609e25239e3eb36532cb33623ba448b028923604c461e{GoooFieldb68bddcca03807b9d3b7351580885098ab19653ee50128d38ae96d5bac276650: "INVALID_INPUT", GoooFieldc673629c5c89a0b38f8954ebea6f889f2292d582cce3fb1be7194fab70ecfad8: "req_name", GoooFieldcb040b8a819e1010951cd9754fd976778e836fe898076d235b2fb3d3080e02cf: false, GoooField9c2e81f8161f0bc825eaa4d4f0f01ecd432178cb7e7db6eee17331a96f8176ac: false, GoooField134fcdefde3b0f35a4f54e2c8857d7f11114b89fdac8465fea0b5a08df5295ea: "INVALID_INPUT"}
	}
	var result = GoooPackage1ActivityCompareServiceGrant(input0, input1, input2, input3)
	return GoooRecorda03acd3ba02b82f739d609e25239e3eb36532cb33623ba448b028923604c461e{GoooFieldb68bddcca03807b9d3b7351580885098ab19653ee50128d38ae96d5bac276650: "VALID_INPUT", GoooFieldc673629c5c89a0b38f8954ebea6f889f2292d582cce3fb1be7194fab70ecfad8: "", GoooFieldcb040b8a819e1010951cd9754fd976778e836fe898076d235b2fb3d3080e02cf: result.GoooFielddbb72b1dd94b1481e9a1dbe9ea75cb83fd7073af74987154593d2dad6adb649d, GoooField9c2e81f8161f0bc825eaa4d4f0f01ecd432178cb7e7db6eee17331a96f8176ac: result.GoooFielda820b495c790dce4367677d8cf1a3fd0e7cff0db75cf63a37389d5c91923c219, GoooField134fcdefde3b0f35a4f54e2c8857d7f11114b89fdac8465fea0b5a08df5295ea: result.GoooField9e32e23e3055b6c63e2d262d3924843e69f7c0c733913e3c8ee8d8f843929c4d}

}

//gooo:generated:end id="gooo://service-policy/checked-impact/evaluate/v1" kind="activity"
//gooo:generated:start id="gooo://service-domain/type" kind="activity"
func GoooPackage2ActivityValidateServiceType(input string) bool {

	var n = int64(len(input))
	if n > 128 {
		return false
	}
	if n == 0 {
		return true
	}
	var first = int64(input[0])
	if !((first >= 65 && first <= 90) || (first >= 97 && first <= 122)) {
		return false
	}
	var offset int64 = 0
	for offset = 0; offset < 128; offset = offset + 1 {
		if offset < n {
			var b = int64(input[offset])
			var alpha = (b >= 65 && b <= 90) || (b >= 97 && b <= 122)
			var digit = b >= 48 && b <= 57
			if !alpha && !digit && b != 43 && b != 45 && b != 46 {
				return false
			}
		}
	}
	return true

}

//gooo:generated:end id="gooo://service-domain/type" kind="activity"
//gooo:generated:start id="gooo://service-domain/name" kind="activity"
func GoooPackage3ActivityValidateServiceName(input string) bool {

	var n = int64(len(input))
	if n > 128 {
		return false
	}
	if n == 0 {
		return true
	}
	var previousDot = true
	var offset int64 = 0
	for offset = 0; offset < 128; offset = offset + 1 {
		if offset < n {
			var b = int64(input[offset])
			if b == 46 {
				if previousDot {
					return false
				}
				previousDot = true
			} else {
				var alpha = (b >= 65 && b <= 90) || (b >= 97 && b <= 122)
				var digit = b >= 48 && b <= 57
				if !alpha && !digit && b != 45 && b != 95 {
					return false
				}
				previousDot = false
			}
		}
	}
	return !previousDot

}

//gooo:generated:end id="gooo://service-domain/name" kind="activity"
//gooo:generated:start id="gooo://service-domain/pattern" kind="activity"
func GoooPackage4ActivityValidateServicePattern(input string) bool {

	var n = int64(len(input))
	if n > 128 {
		return false
	}
	if input == "" || input == "*" {
		return true
	}
	var split int64 = -1
	var offset int64 = 0
	for offset = 0; offset < 128; offset = offset + 1 {
		var end int64 = offset + 3
		if end <= n {
			if input[offset:end] == "://" {
				if split == -1 {
					split = offset
				}
			}
		}
	}
	if split <= 0 {
		return false
	}
	var typ = input[0:split]
	if !GoooPackage2ActivityValidateServiceType(typ) {
		return false
	}
	var start int64 = split + 3
	var part = input[start:n]
	var size = int64(len(part))
	if size == 0 {
		return false
	}
	if part == "*" {
		return true
	}
	if size >= 2 {
		if part[0:2] == "*." {
			part = part[2:size]
		} else {
			var tail = size - 2
			if part[tail:size] == ".*" {
				part = part[0:tail]
			}
		}
	}
	if part == "" {
		return false
	}
	return GoooPackage3ActivityValidateServiceName(part)

}

//gooo:generated:end id="gooo://service-domain/pattern" kind="activity"
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
