// Task fixtures for edit-bench (HASHLINE.md §7). Each task is a small Go
// module the harness copies to a temp dir, with one mutation already applied
// that breaks `go test ./...`; the model is told only "make go test pass".
package main

type task struct {
	name     string
	files    map[string]string // path -> content, written to the run dir
	brokenGo string            // path of the file carrying the mutation
}

func tasks() []task {
	mainHeader := "package main\n\nimport \"fmt\"\n\n"
	testHeader := "package main\n\nimport \"testing\"\n\n"

	mk := func(fn, body string) string { return mainHeader + fn + body }

	fixtures := []task{
		{
			name: "flipped-comparison",
			files: map[string]string{
				"main.go":      mk("func Clamp(v, lo, hi int) int {\n\tif v < lo {\n\t\treturn hi\n\t}\n\tif v > hi {\n\t\treturn lo\n\t}\n\treturn v\n}\n\nfunc main() { fmt.Println(Clamp(5, 0, 10)) }\n", ""),
				"main_test.go": testHeader + "func TestClamp(t *testing.T) {\n\tif Clamp(-1, 0, 10) != 0 {\n\t\tt.Fatal(\"low\")\n\t}\n\tif Clamp(11, 0, 10) != 10 {\n\t\tt.Fatal(\"high\")\n\t}\n}\n",
			},
			brokenGo: "main.go",
		},
		{
			name: "wrong-constant",
			files: map[string]string{
				"main.go":      "package main\n\nconst DaysPerWeek = 8\n\nfunc Weeks(days int) int { return days / DaysPerWeek }\n\nfunc main() { println(Weeks(14)) }\n",
				"main_test.go": testHeader + "func TestWeeks(t *testing.T) {\n\tif Weeks(21) != 3 {\n\t\tt.Fatal(Weeks(21))\n\t}\n}\n",
			},
			brokenGo: "main.go",
		},
		{
			name: "off-by-one",
			files: map[string]string{
				"main.go":      mk("func SumTo(n int) int {\n\ts := 0\n\tfor i := 0; i < n; i++ {\n\t\ts += i\n\t}\n\treturn s\n}\n\nfunc main() { fmt.Println(SumTo(4)) }\n", ""),
				"main_test.go": testHeader + "func TestSumTo(t *testing.T) {\n\tif SumTo(4) != 10 {\n\t\tt.Fatal(SumTo(4))\n\t}\n}\n",
			},
			brokenGo: "main.go",
		},
		{
			name: "missing-nil-check",
			files: map[string]string{
				"main.go":      mk("func First(s []int) int { return s[0] }\n\nfunc main() { fmt.Println(First([]int{1})) }\n", ""),
				"main_test.go": testHeader + "func TestFirst(t *testing.T) {\n\tif First(nil) != 0 {\n\t\tt.Fatal(\"nil\")\n\t}\n\tif First([]int{7}) != 7 {\n\t\tt.Fatal(\"seven\")\n\t}\n}\n",
			},
			brokenGo: "main.go",
		},
		{
			name: "wrong-call",
			files: map[string]string{
				"main.go":      mk("func double(v int) int { return v * 2 }\nfunc triple(v int) int { return v * 3 }\n\nfunc Six() int { return double(2) }\n\nfunc main() { fmt.Println(Six()) }\n", ""),
				"main_test.go": testHeader + "func TestSix(t *testing.T) {\n\tif Six() != 6 {\n\t\tt.Fatal(Six())\n\t}\n}\n",
			},
			brokenGo: "main.go",
		},
		{
			name: "inverted-condition",
			files: map[string]string{
				"main.go":      mk("func Even(n int) bool {\n\tif n%2 == 1 {\n\t\treturn true\n\t}\n\treturn false\n}\n\nfunc main() { fmt.Println(Even(2)) }\n", ""),
				"main_test.go": testHeader + "func TestEven(t *testing.T) {\n\tif !Even(4) {\n\t\tt.Fatal(\"four\")\n\t}\n\tif Even(5) {\n\t\tt.Fatal(\"five\")\n\t}\n}\n",
			},
			brokenGo: "main.go",
		},
		{
			name: "swapped-args",
			files: map[string]string{
				"main.go":      mk("func Pow(base, exp int) int {\n\tr := 1\n\tfor i := 0; i < exp; i++ {\n\t\tr *= base\n\t}\n\treturn r\n}\n\nfunc Cube(base int) int { return Pow(base, 2) }\n\nfunc main() { fmt.Println(Cube(2)) }\n", ""),
				"main_test.go": testHeader + "func TestCube(t *testing.T) {\n\tif Cube(3) != 27 {\n\t\tt.Fatal(Cube(3))\n\t}\n}\n",
			},
			brokenGo: "main.go",
		},
		{
			name: "wrong-return-var",
			files: map[string]string{
				"main.go":      mk("func Scale(v, by int) (int, int) {\n\tscaled := v * by\n\tshifted := v + by\n\treturn v, shifted\n}\n\nfunc main() { fmt.Println(Scale(3, 2)) }\n", ""),
				"main_test.go": testHeader + "func TestScale(t *testing.T) {\n\ts, sh := Scale(3, 2)\n\tif s != 6 || sh != 5 {\n\t\tt.Fatal(s, sh)\n\t}\n}\n",
			},
			brokenGo: "main.go",
		},
		{
			name: "wrong-operator",
			files: map[string]string{
				"main.go":      mk("func Avg(a, b int) int { return (a + b) / 3 }\n\nfunc main() { fmt.Println(Avg(3, 5)) }\n", ""),
				"main_test.go": testHeader + "func TestAvg(t *testing.T) {\n\tif Avg(3, 5) != 4 {\n\t\tt.Fatal(Avg(3, 5))\n\t}\n}\n",
			},
			brokenGo: "main.go",
		},
		{
			name: "boundary-index",
			files: map[string]string{
				"main.go":      mk("func Last(s []int) int { return s[len(s)] }\n\nfunc main() { fmt.Println(Last([]int{1, 2})) }\n", ""),
				"main_test.go": testHeader + "func TestLast(t *testing.T) {\n\tif Last([]int{1, 9}) != 9 {\n\t\tt.Fatal(\"last\")\n\t}\n}\n",
			},
			brokenGo: "main.go",
		},
		{
			name: "wrong-format-verb",
			files: map[string]string{
				"main.go":      mk("func Label(name string, n int) string { return fmt.Sprintf(\"%d: %d\", n, name) }\n\nfunc main() { fmt.Println(Label(\"a\", 1)) }\n", ""),
				"main_test.go": testHeader + "func TestLabel(t *testing.T) {\n\tif Label(\"x\", 2) != \"x: 2\" {\n\t\tt.Fatal(Label(\"x\", 2))\n\t}\n}\n",
			},
			brokenGo: "main.go",
		},
		{
			name: "unexported-call",
			files: map[string]string{
				"main.go":      mk("func hidden() string { return \"ok\" }\nfunc Visible() string { return hidden() }\n\nfunc main() { fmt.Println(Visible()) }\n", ""),
				"main_test.go": testHeader + "func TestVisible(t *testing.T) {\n\tif visible() != \"ok\" {\n\t\tt.Fatal(Visible())\n\t}\n}\n",
			},
			brokenGo: "main_test.go",
		},
		{
			name: "off-by-one-slice",
			files: map[string]string{
				"main.go":      mk("func Middle(s []int) []int { return s[1 : len(s)-2] }\n\nfunc main() { fmt.Println(Middle([]int{1, 2, 3})) }\n", ""),
				"main_test.go": testHeader + "func TestMiddle(t *testing.T) {\n\tgot := Middle([]int{1, 2, 3, 4})\n\tif got[0] != 2 || got[len(got)-1] != 3 {\n\t\tt.Fatal(got)\n\t}\n}\n",
			},
			brokenGo: "main.go",
		},
		{
			name: "wrong-default",
			files: map[string]string{
				"main.go":      mk("func Greet(name string) string {\n\tif name == \"\" {\n\t\treturn \"hi, stranger\"\n\t}\n\treturn \"bye, \" + name\n}\n\nfunc main() { fmt.Println(Greet(\"a\")) }\n", ""),
				"main_test.go": testHeader + "func TestGreet(t *testing.T) {\n\tif Greet(\"\") != \"hi, stranger\" {\n\t\tt.Fatal(Greet(\"\"))\n\t}\n\tif Greet(\"sam\") != \"hi, sam\" {\n\t\tt.Fatal(Greet(\"sam\"))\n\t}\n}\n",
			},
			brokenGo: "main.go",
		},
		{
			name: "shadowed-var",
			files: map[string]string{
				"main.go":      mk("func Double(v int) int {\n\tdoubled := v * 2\n\tif v > 100 {\n\t\tdoubled := v\n\t\t_ = doubled\n\t}\n\treturn v\n}\n\nfunc main() { fmt.Println(Double(2)) }\n", ""),
				"main_test.go": testHeader + "func TestDouble(t *testing.T) {\n\tif Double(3) != 6 {\n\t\tt.Fatal(Double(3))\n\t}\n}\n",
			},
			brokenGo: "main.go",
		},
		{
			name: "missing-break",
			files: map[string]string{
				"main.go":      mk("func Size(s string) string {\n\tswitch {\n\tcase len(s) < 3:\n\t\treturn \"small\"\n\tcase len(s) < 6:\n\t\treturn \"tiny\"\n\tdefault:\n\t\treturn \"big\"\n\t}\n}\n\nfunc main() { fmt.Println(Size(\"abcdef\")) }\n", ""),
				"main_test.go": testHeader + "func TestSize(t *testing.T) {\n\tif Size(\"ab\") != \"small\" {\n\t\tt.Fatal(Size(\"ab\"))\n\t}\n\tif Size(\"abcd\") != \"medium\" {\n\t\tt.Fatal(Size(\"abcd\"))\n\t}\n}\n",
			},
			brokenGo: "main.go",
		},
		{
			name: "wrong-map-key",
			files: map[string]string{
				"main.go":      mk("func Count(s string) map[string]int {\n\tm := map[string]int{}\n\tfor _, r := range s {\n\t\tm[\"x\"]++\\n\\t\\t_ = r\n\t}\n\treturn m\n}\n\nfunc main() { fmt.Println(Count(\"aabb\")) }\n", ""),
				"main_test.go": testHeader + "func TestCount(t *testing.T) {\n\tm := Count(\"aab\")\n\tif m[\"a\"] != 2 || m[\"b\"] != 1 {\n\t\tt.Fatal(m)\n\t}\n}\n",
			},
			brokenGo: "main_test.go",
		},
		{
			name: "nil-map-write",
			files: map[string]string{
				"main.go":      mk("func Safe(m map[string]int, k string) int {\n\tm[k] = 1\n\treturn m[k]\n}\n\nfunc main() { fmt.Println(Safe(map[string]int{}, \"a\")) }\n", ""),
				"main_test.go": testHeader + "func TestSafe(t *testing.T) {\n\tif Safe(nil, \"x\") != 1 {\n\t\tt.Fatal(\"safe\")\n\t}\n}\n",
			},
			brokenGo: "main.go",
		},
		{
			name: "wrong-field",
			files: map[string]string{
				"main.go":      "package main\n\nimport \"fmt\"\n\ntype P struct{ Name, Title string }\n\nfunc Greet(p P) string { return \"hi \" + p.Title }\n\nfunc main() { fmt.Println(Greet(P{Name: \"a\", Title: \"b\"})) }\n",
				"main_test.go": testHeader + "func TestGreet(t *testing.T) {\n\tif Greet(P{Name: \"sam\", Title: \"x\"}) != \"hi sam\" {\n\t\tt.Fatal(Greet(P{Name: \"sam\"}))\n\t}\n}\n",
			},
			brokenGo: "main.go",
		},
		{
			name: "wrong-compare-str",
			files: map[string]string{
				"main.go":      mk("func IsYes(s string) bool { return s == \"y\" && s == \"yes\" }\n\nfunc main() { fmt.Println(IsYes(\"yes\")) }\n", ""),
				"main_test.go": testHeader + "func TestIsYes(t *testing.T) {\n\tif !IsYes(\"yes\") {\n\t\tt.Fatal(\"yes\")\n\t}\n\tif IsYes(\"no\") {\n\t\tt.Fatal(\"no\")\n\t}\n}\n",
			},
			brokenGo: "main.go",
		},
	}
	return fixtures
}
