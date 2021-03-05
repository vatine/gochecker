package main

import (
	"flag"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/vatine/gochecker/pkg/pkgdata"
)

type accumulator struct {
	seen                     float64
	downloadFailed           float64
	buildSuccess             float64
	testSuccess              float64
	noTestTargets            float64
	allVetsPassed            float64
	allFmtOK                 float64
	buildTargets             []float64
	buildFractions           []float64
	testTargets              []float64
	testFractions            []float64
	vetFractions             []float64
	buildTargetsFailed       []float64
	buildTargetsFmtFailed    []float64
	testTargetsFailed        []float64
	vetTargetsFailed         []float64
	failedBuildTargetsFailed []float64
	failedTestTargetsFailed  []float64
	passedBuildFailedTests   []float64
	versionCount             map[string]int64
}

type mostData struct {
	name  string
	count int64
}

type nMost struct {
	max  int
	seen int
	data []mostData
}

func moduleFromPackage(pkg string) string {
	return strings.Split(pkg, "@")[0]
}

func (n *nMost) Len() int {
	return len(n.data)
}

func (n *nMost) Less(i, j int) bool {
	return n.data[i].count > n.data[j].count
}

func (n *nMost) Swap(i, j int) {
	n.data[j], n.data[i] = n.data[i], n.data[j]
}

func (n *nMost) observe(name string, count int64) {
	if n.seen < n.max {
		n.data[n.seen] = mostData{name, count}
		n.seen++
		return
	}

	n.data[n.max] = mostData{name, count}
	sort.Sort(n)
}

func newMostN(n int) *nMost {
	var rv nMost

	rv.max = n
	rv.data = make([]mostData, n+1, n+1)

	return &rv
}

// Process a single package into an accumulator
func (a *accumulator) process(p pkgdata.PackageStats) {
	a.seen += 1.0

	if !p.DownloadSucceeded {
		a.downloadFailed += 1.0
		return
	}

	buildFraction := 1.0
	testFraction := 1.0
	failedBuildCount := float64(len(p.FailedBuilds))
	failedTestCount := float64(len(p.FailedTests))
	failedFmtCount := float64(len(p.FailedFmt))

	if p.TestableTargets == 0 {
		a.noTestTargets += 1.0
	}

	a.buildTargets = append(a.buildTargets, float64(p.BuildableTargets))
	a.testTargets = append(a.buildTargets, float64(p.TestableTargets))

	if !p.AllBuildsPass {
		builds := float64(p.BuildableTargets)
		buildFraction = (builds - float64(len(p.FailedBuilds))) / builds
		a.failedBuildTargetsFailed = append(a.failedBuildTargetsFailed, failedBuildCount)
	} else {
		a.buildSuccess += 1.0
		a.failedBuildTargetsFailed = append(a.failedBuildTargetsFailed, 0.0)
	}

	if !p.AllTestsPassed {
		tests := float64(p.TestableTargets)
		testFraction = (tests - failedTestCount) / tests
		a.failedTestTargetsFailed = append(a.failedTestTargetsFailed, failedTestCount)
		if p.AllBuildsPass {
			a.passedBuildFailedTests = append(a.passedBuildFailedTests, failedTestCount)
		}
	} else {
		a.testSuccess += 1.0
	}

	a.buildFractions = append(a.buildFractions, buildFraction)
	a.buildTargetsFailed = append(a.buildTargetsFailed)
	a.buildTargetsFmtFailed = append(a.buildTargetsFmtFailed, failedFmtCount)

	a.testFractions = append(a.testFractions, testFraction)
	a.testTargetsFailed = append(a.testTargetsFailed, float64(len(p.FailedTests)))

	passedVetCount := float64(len(p.VetPassed))
	failedVetCount := float64(len(p.FailedVets))
	a.vetFractions = append(a.vetFractions, passedVetCount/(passedVetCount+failedVetCount))
	if (passedVetCount > 0.0) && (failedVetCount) == 0 {
		a.allVetsPassed += 1.0
	}
	if failedFmtCount == 0 {
		a.allFmtOK += 1.0
	}
	a.vetTargetsFailed = append(a.vetTargetsFailed, float64(failedVetCount))
}

func meanAndDev(data []float64) (mean, stddev float64) {
	return meanAndDevInner(data, false)
}

func meanAndDevNoZeroes(data []float64) (mean, stddev float64) {
	return meanAndDevInner(data, true)
}

// Returns the mean and standard deviation of a []float64
func meanAndDevInner(data []float64, excludeZero bool) (mean, stddev float64) {
	acc := 0.0
	count := 0.0

	for _, v := range data {
		if excludeZero && v == 0.0 {
			continue
		}
		acc += v
		count += 1.0
	}

	mean = acc / count

	acc2 := 0.0

	for _, v := range data {
		if excludeZero && v == 0.0 {
			continue
		}
		delta := (v - mean)
		acc2 += (delta * delta)
	}

	switch {
	case count == 0.0:
		return 0.0, 0.0
	case count == 1.0:
		return mean, 0.0
	}

	stddev = math.Sqrt(acc2 / (count - 1.0))
	return mean, stddev
}

func newAccumulator() *accumulator {
	var rv accumulator

	rv.versionCount = make(map[string]int64)

	return &rv
}

// Process a batch of package data, return two accumulators, one for
// successful and one for failed downloads.
func statsRun() (accumulator, accumulator) {
	pkgChan := pkgdata.AllPackages()
	rv := newAccumulator()
	fails := newAccumulator()

	for data := range pkgChan {
		if data.Stats.DownloadSucceeded {
			rv.versionCount[moduleFromPackage(data.Name)] += 1
			rv.process(data.Stats)
		} else {
			rv.downloadFailed += 1.0
			fails.versionCount[moduleFromPackage(data.Name)] += 1
			fails.process(data.Stats)
		}
	}

	return *rv, *fails
}

// Return how much frac is of base, as a percentage
func percent(base, frac float64) float64 {
	return (frac / base) * 100.0
}

// Return median, 75th, 90tyhm 95th percentiles of incoming data
func percentiles(data []float64) (median, pct75, pct90, pct95, pct99, max float64) {
	count := len(data)

	tmp := make([]float64, count)
	copy(tmp, data)
	sort.Float64s(tmp)

	medianIx := count / 2
	pct75Ix := (75 * count) / 100
	pct90Ix := (90 * count) / 100
	pct95Ix := (95 * count) / 100
	pct99Ix := (99 * count) / 100

	return tmp[medianIx], tmp[pct75Ix], tmp[pct90Ix], tmp[pct95Ix], tmp[pct99Ix], tmp[count-1]
}

// Return the N most frequent modules, as a []mostData
func (a accumulator) mostFrequentModules(n int) []mostData {
	most := newMostN(n)

	for pkg, count := range a.versionCount {
		most.observe(pkg, count)
	}

	return most.data[0:n]
}

// Outputs a LaTeX table with "just build statistics"
func (a accumulator) emitBuildStats() {
	fmt.Println(`\begin{table}[ht]`)
	fmt.Println(`\caption{Build target statistics}`)
	fmt.Println(`\label{table:build}`)
	fmt.Println(`\begin{tabular}{|l|r|}`)
	fmt.Println(` \hline`)

	fmt.Printf(`  Packages processed & %.0f \\`, a.seen)
	fmt.Println()
	fmt.Printf(`  Packages failed to download & %.0f \\`, a.downloadFailed)
	fmt.Println()

	fmt.Printf(`  No build failures & %.0f (%f\%%) \\`, a.buildSuccess, percent(a.seen, a.buildSuccess))
	fmt.Println()

	fmt.Printf(`  No vet failures & %.0f (%f\%%) \\`, a.allVetsPassed, percent(a.seen, a.allVetsPassed))
	fmt.Println()

	fmt.Printf(`  No fmt failures & %.0f (%f\%%) \\`, a.allFmtOK, percent(a.seen, a.allFmtOK))
	fmt.Println()

	fmt.Printf(`  No test targets & %.0f (%f\%%) \\`, a.noTestTargets, percent(a.seen, a.noTestTargets))
	fmt.Println()

	fmt.Println(` \hline`)
	mean, dev := meanAndDev(a.buildTargets)
	median, pct75, pct90, pct95, pct99, pct100 := percentiles(a.buildTargets)
	fmt.Printf(`  Mean build targets (all modules)& %f \\`, mean)
	fmt.Println()
	fmt.Printf(`  stddev & %f \\`, dev)
	fmt.Println()
	fmt.Printf(`  Median build targets & %.0f \\`, median)
	fmt.Println()
	fmt.Printf(`  75th percentile \# of build targets & %.0f \\`, pct75)
	fmt.Println()
	fmt.Printf(`  90th percentile \# of build targets & %.0f \\`, pct90)
	fmt.Println()
	fmt.Printf(`  95th percentile \# of build targets & %.0f \\`, pct95)
	fmt.Println()
	fmt.Printf(`  99th percentile \# of build targets & %.0f \\`, pct99)
	fmt.Println()
	fmt.Printf(`  Max \# of build targets & %.0f \\`, pct100)
	fmt.Println()

	fmt.Println(` \hline`)
	mean, dev = meanAndDevNoZeroes(a.buildTargets)
	fmt.Printf(`  Mean build targets (at least one buildable)& %f \\`, mean)
	fmt.Println()
	fmt.Printf(`  stddev & %f \\`, dev)
	fmt.Println()

	fmt.Println(` \hline`)
	mean, dev = meanAndDev(a.failedBuildTargetsFailed)
	fmt.Printf(`  Mean failed build targets (all modules)& %f \\`, mean)
	fmt.Println()
	fmt.Printf(`  stddev & %f \\`, dev)
	fmt.Println()

	fmt.Println(` \hline`)
	mean, dev = meanAndDevNoZeroes(a.failedBuildTargetsFailed)
	fmt.Printf(`  Mean failed build targets (at least one failed)& %f \\`, mean)
	fmt.Println()
	fmt.Printf(`  stddev & %f \\`, dev)
	fmt.Println()

	fmt.Println(` \hline`)
	mean, dev = meanAndDev(a.vetTargetsFailed)
	fmt.Printf(`  Mean failed vet targets (all modules)& %f \\`, mean)
	fmt.Println()
	fmt.Printf(`  stddev & %f \\`, dev)
	fmt.Println()

	fmt.Println(` \hline`)
	mean, dev = meanAndDevNoZeroes(a.vetTargetsFailed)
	fmt.Printf(`  Mean failed vet targets (at least one failed)& %f \\`, mean)
	fmt.Println()
	fmt.Printf(`  stddev & %f \\`, dev)
	fmt.Println()

	fmt.Println(` \hline`)
	fmt.Println(`\end{tabular}`)
	fmt.Println(`\end{table}`)

}

// Outputs a LaTeX table with "just test statistics"
func (a accumulator) emitTestStats() {
	fmt.Println(`\begin{table}[ht]`)
	fmt.Println(`\caption{Test target statistics}`)
	fmt.Println(`\label{table:test}`)
	fmt.Println(`\begin{tabular}{|l|r|}`)
	fmt.Println(` \hline`)

	fmt.Printf(`  Packages seen & %.0f \\`, a.seen)
	fmt.Println()

	fmt.Printf(`  No test failures & %.0f (%f\%%) \\`, a.testSuccess, percent(a.seen, a.testSuccess))
	fmt.Println()
	fmt.Printf(`  No test failures (with tests) & %.0f (%f\%%) \\`, a.testSuccess-a.noTestTargets, percent(a.seen-a.noTestTargets, a.testSuccess-a.noTestTargets))
	fmt.Println()
	passedBuildFailedTests := float64(len(a.passedBuildFailedTests))
	fmt.Printf(`  No build failures, but test failures & %.0f (%f\%%) \\`, passedBuildFailedTests, percent(a.seen, passedBuildFailedTests))
	fmt.Println()

	fmt.Printf(`  No tests & %.0f (%f\%%) \\`, a.noTestTargets, percent(a.seen, a.noTestTargets))
	fmt.Println()

	fmt.Println(` \hline`)

	mean, dev := meanAndDev(a.passedBuildFailedTests)
	fmt.Printf(`  Mean failed test targets for passed builds (all) & %f \\`, mean)
	fmt.Println()
	fmt.Printf(`  stddev & %f \\`, dev)
	fmt.Println()
	fmt.Println(` \hline`)

	mean, dev = meanAndDevNoZeroes(a.passedBuildFailedTests)
	fmt.Printf(`  Mean failed test targets for passed builds (at least one fail) & %f \\`, mean)
	fmt.Println()
	fmt.Printf(`  stddev & %f \\`, dev)
	fmt.Println()
	fmt.Println(` \hline`)

	mean, dev = meanAndDev(a.failedTestTargetsFailed)
	fmt.Printf(`  Mean failed test targets, all packages& %f \\`, mean)
	fmt.Println()
	fmt.Printf(`  stddev & %f \\`, dev)
	fmt.Println()
	fmt.Println(` \hline`)

	mean, dev = meanAndDevNoZeroes(a.failedTestTargetsFailed)
	fmt.Printf(`  Mean failed test targets, packages with at least one test failure& %f \\`, mean)
	fmt.Println()
	fmt.Printf(`  stddev & %f \\`, dev)
	fmt.Println()
	fmt.Println(` \hline`)

	fmt.Println(`\end{tabular}`)
	fmt.Println(`\end{table}`)
}

func (a accumulator) emitVersionTable(n int, fail bool) {
	most := a.mostFrequentModules(n)
	failMsg := ""
	label := "table:versions"
	if fail {
		failMsg = "fail to"
		label = "table:failversions"
	}
	fmt.Println(`\begin{table}[ht]`)
	fmt.Printf(`\caption{Most versions per module that %s download}`, failMsg)
	fmt.Println()
	fmt.Printf(`\label{%s}`, label)
	fmt.Println()
	fmt.Println(`\begin{tabular}{|l|r|}`)
	fmt.Println(`\hline`)
	for _, data := range most {
		fmt.Printf(" %s & %d \\\\\n", data.name, data.count)
	}
	fmt.Println(`\hline`)
	fmt.Println(`\end{tabular}`)
	fmt.Println(`\end{table}`)

}

func statsTables() {
	acc, fails := statsRun()

	acc.emitBuildStats()
	fmt.Println()
	acc.emitTestStats()
	fmt.Println()
	acc.emitVersionTable(10, false)

	fails.emitVersionTable(10, true)
}

func main() {
	var dataDir string

	logrus.SetLevel(logrus.WarnLevel)

	flag.StringVar(&dataDir, "datadir", "/tmp/go_data", "Data directory for long-term storage.")

	flag.Parse()

	pkgdata.SetStoragePath(dataDir)
	pkgdata.LoadLatest()

	statsTables()
}
