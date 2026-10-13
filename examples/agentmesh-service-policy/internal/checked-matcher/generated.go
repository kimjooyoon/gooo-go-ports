package gooo_workspace

//gooo:generated:start id="gooo://service-policy/checked-matcher/decision/v1" kind="entity"
type GoooRecord83708ca28c6cca08c61afe1fb49f104432988292e0d50ec469a1446edf73cb84 struct {
	GoooFielde3471a0e692d31ef618ef2e3bc14f62168e9707aa4e6d55c51e8ca4edc8eb633 string `json:"domain_status"`
	GoooFielda97eca0e6b3cc33b5d8e03452ba3302e8e1a4516bb02eef7afad70d871cb7f38 string `json:"invalid_field"`
	GoooField73bb1b90ec5f047d4a412aa3d5d3003e1fb3e78e956822e930758627c329a0dc bool   `json:"matched"`
}

//gooo:generated:end id="gooo://service-policy/checked-matcher/decision/v1" kind="entity"
//gooo:generated:start id="gooo://service-policy/checked-matcher/evaluate/v1" kind="activity"
func GoooPackage4ActivityMatchServicePatternChecked(input0 string, input1 string, input2 string) GoooRecord83708ca28c6cca08c61afe1fb49f104432988292e0d50ec469a1446edf73cb84 {

	if !GoooPackage3ActivityValidateServicePattern(input0) {
		return GoooRecord83708ca28c6cca08c61afe1fb49f104432988292e0d50ec469a1446edf73cb84{GoooFielde3471a0e692d31ef618ef2e3bc14f62168e9707aa4e6d55c51e8ca4edc8eb633: "INVALID_INPUT", GoooFielda97eca0e6b3cc33b5d8e03452ba3302e8e1a4516bb02eef7afad70d871cb7f38: "pattern", GoooField73bb1b90ec5f047d4a412aa3d5d3003e1fb3e78e956822e930758627c329a0dc: false}
	}
	if !GoooPackage1ActivityValidateServiceType(input1) {
		return GoooRecord83708ca28c6cca08c61afe1fb49f104432988292e0d50ec469a1446edf73cb84{GoooFielde3471a0e692d31ef618ef2e3bc14f62168e9707aa4e6d55c51e8ca4edc8eb633: "INVALID_INPUT", GoooFielda97eca0e6b3cc33b5d8e03452ba3302e8e1a4516bb02eef7afad70d871cb7f38: "req_type", GoooField73bb1b90ec5f047d4a412aa3d5d3003e1fb3e78e956822e930758627c329a0dc: false}
	}
	if !GoooPackage2ActivityValidateServiceName(input2) {
		return GoooRecord83708ca28c6cca08c61afe1fb49f104432988292e0d50ec469a1446edf73cb84{GoooFielde3471a0e692d31ef618ef2e3bc14f62168e9707aa4e6d55c51e8ca4edc8eb633: "INVALID_INPUT", GoooFielda97eca0e6b3cc33b5d8e03452ba3302e8e1a4516bb02eef7afad70d871cb7f38: "req_name", GoooField73bb1b90ec5f047d4a412aa3d5d3003e1fb3e78e956822e930758627c329a0dc: false}
	}
	var result = GoooPackage0ActivityMatchServicePattern(input0, input1, input2)
	return GoooRecord83708ca28c6cca08c61afe1fb49f104432988292e0d50ec469a1446edf73cb84{GoooFielde3471a0e692d31ef618ef2e3bc14f62168e9707aa4e6d55c51e8ca4edc8eb633: "VALID_INPUT", GoooFielda97eca0e6b3cc33b5d8e03452ba3302e8e1a4516bb02eef7afad70d871cb7f38: "", GoooField73bb1b90ec5f047d4a412aa3d5d3003e1fb3e78e956822e930758627c329a0dc: result}

}

//gooo:generated:end id="gooo://service-policy/checked-matcher/evaluate/v1" kind="activity"
//gooo:generated:start id="gooo://service-domain/type" kind="activity"
func GoooPackage1ActivityValidateServiceType(input string) bool {

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
func GoooPackage2ActivityValidateServiceName(input string) bool {

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
func GoooPackage3ActivityValidateServicePattern(input string) bool {

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
	if !GoooPackage1ActivityValidateServiceType(typ) {
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
	return GoooPackage2ActivityValidateServiceName(part)

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
