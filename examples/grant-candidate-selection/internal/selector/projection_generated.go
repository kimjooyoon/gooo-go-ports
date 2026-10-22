package selector

//gooo:generated:start id="gooo://service-policy/candidate-selection/input/v1" kind="entity"
type GoooRecord42b864fa50cb6ccd0a85ce0b0357134798e893f0448cd3a7d70d00892d3bf0b0 struct {
	GoooField3e36a565fb10eda4b413ec4503dc96b47cca3efdd15609bdcf6b20b64ba414f4 string   `json:"phase"`
	GoooFieldca17fb3c3cdf4c28ad7f9d3f50ee22bfe932f1ff7f8cf0443411001a8874b0f7 string   `json:"context_id"`
	GoooField05be2b7d3bbbf03fd071448f485ed992800d02331a6d849d3b916080b7fb3010 []string `json:"candidate_ids"`
	GoooField5c4dab7bebe2a63bde47995f43d06b46b085ef9863325780448f69fc1ab60671 []string `json:"assessment_ids"`
	GoooFieldfa3fe6437a3accaf6087c122911deee785361c04c0b12f827fa03881dececcd2 []string `json:"context_ids"`
	GoooFielda91703fa4739c7aab0f4b195f4078ddd2a04e89cca320d124be2ac6db2d1e644 []string `json:"statuses"`
	GoooFieldfa9ce851f769a68f131c052345591290f4e9341e3691d956196ef6420f79799e []int64  `json:"priorities"`
	GoooField2561eb0e807a6d03b849c06baa0b3ca772c9bcd60a301be7e4394784408b0f00 []bool   `json:"bindings_verified"`
}

//gooo:generated:end id="gooo://service-policy/candidate-selection/input/v1" kind="entity"
//gooo:generated:start id="gooo://service-policy/candidate-selection/result/v1" kind="entity"
type GoooRecordfb9100b3a544a011eaad15a90ce25aa78b43539dbaa7de19aecfd85616d6eeef struct {
	GoooFieldd222a9375fa35992acd9018e436b4381d432910dd91d670bcb8bc50e8d0dd497 string `json:"context_id"`
	GoooField97fdae3df3f9c4985d8e2a61f75113b9e95b9d8b36302eeb9b89cf05cf37b3f9 string `json:"state"`
	GoooField9047500c625a50d55bf4d6bf58ea60fb47fa0995f15cf8f090cd530391995a75 string `json:"reason"`
	GoooFieldbfeccd1f811c9942230a2c03914267fcb6c19520dc11928c1f723ede73fe6b66 string `json:"selected_candidate_id"`
	GoooField0e984ad6c2532a7829736502fe16b8a7743e0dbacef8d260d14588ea97362942 string `json:"selected_assessment_id"`
	GoooFieldf04dad0bb349e4bedcba1364abbeb7b2b3b99159ca6ac9b0dcccd9b6e3f738a7 int64  `json:"selected_index"`
	GoooFieldf3daab021a38c3134c4e1016cf17b8c498ed1a75acf9857d43084adc8b37c4d2 int64  `json:"selected_priority"`
	GoooFieldcf3497e87cc0b98cd10f2a98e08e05fd36b98d5f5f87d044704e80fb5a63a064 int64  `json:"candidate_count"`
	GoooFieldee0d738147efaff9ab159657b556523f31dc457eacb0631d262f73586a32d6eb int64  `json:"eligible_count"`
	GoooField73584955536b2b8c1accda24e675c6ba325174e05c5984ca4b1fee386de4ac50 int64  `json:"invalid_index"`
	GoooField3513a6d70f27f8f71c0fa35d83b08eafa1b3ceffe03831ccd6fbaa53872c4805 int64  `json:"unknown_index"`
	GoooFielde7f3a9e13a5d0d063988817e06f09368954e8b1d17b26a58e2c2a26b7fc665ff bool   `json:"execution_authorized"`
}

//gooo:generated:end id="gooo://service-policy/candidate-selection/result/v1" kind="entity"

//gooo:generated:start id="gooo://service-policy/candidate-selection/select/v1" kind="activity"
func GoooComposedActivity0(input GoooRecord42b864fa50cb6ccd0a85ce0b0357134798e893f0448cd3a7d70d00892d3bf0b0) GoooRecordfb9100b3a544a011eaad15a90ce25aa78b43539dbaa7de19aecfd85616d6eeef {

	var count = int64(len(input.GoooField05be2b7d3bbbf03fd071448f485ed992800d02331a6d849d3b916080b7fb3010))
	var invalid = false
	var invalidIndex int64 = -1
	var unknown int64 = -1
	var selected = ""
	var assessment = ""
	var selectedIndex int64 = -1
	var priority int64 = 0
	var eligible int64 = 0
	if input.GoooFieldca17fb3c3cdf4c28ad7f9d3f50ee22bfe932f1ff7f8cf0443411001a8874b0f7 == "" || count < 2 || count > 32 || int64(len(input.GoooFieldfa9ce851f769a68f131c052345591290f4e9341e3691d956196ef6420f79799e)) != count ||
		(input.GoooField3e36a565fb10eda4b413ec4503dc96b47cca3efdd15609bdcf6b20b64ba414f4 != "ADMIT" && input.GoooField3e36a565fb10eda4b413ec4503dc96b47cca3efdd15609bdcf6b20b64ba414f4 != "SELECT") {
		invalid = true
	} else {
		var index int64 = 0
		for _, candidate := range input.GoooField05be2b7d3bbbf03fd071448f485ed992800d02331a6d849d3b916080b7fb3010 {
			var bad = candidate == "" || len(candidate) > 128 || input.GoooFieldfa9ce851f769a68f131c052345591290f4e9341e3691d956196ef6420f79799e[index] < 0
			var prior int64 = 0
			for _, previous := range input.GoooField05be2b7d3bbbf03fd071448f485ed992800d02331a6d849d3b916080b7fb3010 {
				if prior < index && previous == candidate {
					bad = true
				}
				prior = prior + 1
			}
			if bad {
				invalid = true
				if invalidIndex == -1 {
					invalidIndex = index
				}
			}
			index = index + 1
		}
	}
	if !invalid && input.GoooField3e36a565fb10eda4b413ec4503dc96b47cca3efdd15609bdcf6b20b64ba414f4 == "SELECT" {
		if int64(len(input.GoooField5c4dab7bebe2a63bde47995f43d06b46b085ef9863325780448f69fc1ab60671)) != count || int64(len(input.GoooFieldfa3fe6437a3accaf6087c122911deee785361c04c0b12f827fa03881dececcd2)) != count ||
			int64(len(input.GoooFielda91703fa4739c7aab0f4b195f4078ddd2a04e89cca320d124be2ac6db2d1e644)) != count || int64(len(input.GoooField2561eb0e807a6d03b849c06baa0b3ca772c9bcd60a301be7e4394784408b0f00)) != count {
			invalid = true
		} else {
			var index int64 = 0
			for _, candidate := range input.GoooField05be2b7d3bbbf03fd071448f485ed992800d02331a6d849d3b916080b7fb3010 {
				var status = input.GoooFielda91703fa4739c7aab0f4b195f4078ddd2a04e89cca320d124be2ac6db2d1e644[index]
				var bad = input.GoooField5c4dab7bebe2a63bde47995f43d06b46b085ef9863325780448f69fc1ab60671[index] == "" ||
					(status != "ELIGIBLE" && status != "REFUSED" && status != "INVALID_INPUT")
				if bad {
					invalid = true
					if invalidIndex == -1 {
						invalidIndex = index
					}
				}
				if unknown == -1 && (!input.GoooField2561eb0e807a6d03b849c06baa0b3ca772c9bcd60a301be7e4394784408b0f00[index] || input.GoooFieldfa3fe6437a3accaf6087c122911deee785361c04c0b12f827fa03881dececcd2[index] != input.GoooFieldca17fb3c3cdf4c28ad7f9d3f50ee22bfe932f1ff7f8cf0443411001a8874b0f7) {
					unknown = index
				}
				if status == "ELIGIBLE" {
					eligible = eligible + 1
					if selected == "" || input.GoooFieldfa9ce851f769a68f131c052345591290f4e9341e3691d956196ef6420f79799e[index] < priority {
						selected = candidate
						assessment = input.GoooField5c4dab7bebe2a63bde47995f43d06b46b085ef9863325780448f69fc1ab60671[index]
						selectedIndex = index
						priority = input.GoooFieldfa9ce851f769a68f131c052345591290f4e9341e3691d956196ef6420f79799e[index]
					}
				}
				index = index + 1
			}
		}
	}
	var state = "REFUSED"
	var reason = "NO_ELIGIBLE_ASSESSMENT"
	if invalid || unknown != -1 {
		selected = ""
		assessment = ""
		selectedIndex = -1
		priority = 0
		eligible = 0
		if invalid {
			state = "INVALID_INPUT"
			reason = "INVALID_CANDIDATE_COLUMNS"
		} else {
			state = "UNKNOWN"
			reason = "UNBOUND_OR_DIFFERENT_CONTEXT"
		}
	} else if input.GoooField3e36a565fb10eda4b413ec4503dc96b47cca3efdd15609bdcf6b20b64ba414f4 == "ADMIT" {
		state = "ADMITTED"
		reason = "CANDIDATE_COLUMNS_ADMITTED"
	} else if selected != "" {
		state = "PROPOSED"
		reason = "LOWEST_SUPPLIED_PRIORITY_STABLE_INPUT_ORDER"
	}
	return GoooRecordfb9100b3a544a011eaad15a90ce25aa78b43539dbaa7de19aecfd85616d6eeef{GoooFieldd222a9375fa35992acd9018e436b4381d432910dd91d670bcb8bc50e8d0dd497: input.GoooFieldca17fb3c3cdf4c28ad7f9d3f50ee22bfe932f1ff7f8cf0443411001a8874b0f7, GoooField97fdae3df3f9c4985d8e2a61f75113b9e95b9d8b36302eeb9b89cf05cf37b3f9: state, GoooField9047500c625a50d55bf4d6bf58ea60fb47fa0995f15cf8f090cd530391995a75: reason,
		GoooFieldbfeccd1f811c9942230a2c03914267fcb6c19520dc11928c1f723ede73fe6b66: selected, GoooField0e984ad6c2532a7829736502fe16b8a7743e0dbacef8d260d14588ea97362942: assessment,
		GoooFieldf04dad0bb349e4bedcba1364abbeb7b2b3b99159ca6ac9b0dcccd9b6e3f738a7: selectedIndex, GoooFieldf3daab021a38c3134c4e1016cf17b8c498ed1a75acf9857d43084adc8b37c4d2: priority, GoooFieldcf3497e87cc0b98cd10f2a98e08e05fd36b98d5f5f87d044704e80fb5a63a064: count,
		GoooFieldee0d738147efaff9ab159657b556523f31dc457eacb0631d262f73586a32d6eb: eligible, GoooField73584955536b2b8c1accda24e675c6ba325174e05c5984ca4b1fee386de4ac50: invalidIndex, GoooField3513a6d70f27f8f71c0fa35d83b08eafa1b3ceffe03831ccd6fbaa53872c4805: unknown,
		GoooFielde7f3a9e13a5d0d063988817e06f09368954e8b1d17b26a58e2c2a26b7fc665ff: false}

}

//gooo:generated:end id="gooo://service-policy/candidate-selection/select/v1" kind="activity"
