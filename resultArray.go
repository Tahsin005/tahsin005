func resultArray(nums []int) []int {
  a1 := []int{nums[0]}
	a2 := []int{nums[1]}

	for i := 2; i < len(nums); i++ {
		if a1[len(a1) - 1] > a2[len(a2) - 1] {
			a1 = append(a1, nums[i])
		} else {
			a2 = append(a2, nums[i])
		}
	}
	return slices.Concat(a1, a2)
}
