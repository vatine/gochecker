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
	seen float64
	downloadFailed float64
	buildSuccess float64
	testSuccess float64
	buildFractions []float64
	testFractions []float64
	buildTargetsFailed []float64
	testTargetsFailed []float64
	failedBuildTargetsFailed []float64
	failedTestTargetsFailed []float64
	passedBuildFailedTests []float64
	versionCount map[string]int64
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
	}

	buildFraction := 1.0
	testFraction := 1.0
	failedBuildCount := float64(len(p.FailedBuilds))
	failedTestCount := float64(len(p.FailedTests))

	if !p.AllBuildsPass {
		builds := float64(p.BuildableTargets)
		buildFraction = (builds - float64(len(p.FailedBuilds))) / builds
		a.failedBuildTargetsFailed = append(a.failedBuildTargetsFailed, failedBuildCount)
	} else {
		a.buildSuccess += 1.0
	}
	
	
	if !p.AllTestsPassed {
		tests := float64(p.TestableTargets)
		testFraction = (tests - float64(len(p.FailedTests))) / tests
		a.failedTestTargetsFailed = append(a.failedTestTargetsFailed, failedTestCount)
		if p.AllBuildsPass {
			a.passedBuildFailedTests = append(a.passedBuildFailedTests, failedTestCount)
		}
	} else {
		a.testSuccess += 1.0
	}

	a.buildFractions = append(a.buildFractions, buildFraction)
	a.buildTargetsFailed = append(a.buildTargetsFailed, )

	a.testFractions = append(a.testFractions, testFraction)
	a.testTargetsFailed = append(a.testTargetsFailed, float64(len(p.FailedTests)))
}


// Returns the mean and standard deviation of a []float64
func meanAndDev(data []float64) (mean, stddev float64) {
	acc := 0.0
	count := float64(len(data))

	for _, v := range data {
		acc += v
	}

	mean = acc / count

	acc2 := 0.0

	for _, v := range data {
		delta := (v - mean)
		acc2 += (delta * delta)
	}

	if count <= 1.0 {
		return mean, 0.0
	} else {
		stddev = math.Sqrt(acc2 / (count - 1.0))
	}

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
			fails.versionCount[moduleFromPackage(data.Name)] += 1
			fails.process(data.Stats)
		}
	}

	return *rv, *fails
}

// Return how much frac is of base, as a percentage 
func percent(base, frac float64) float64 {
	return (frac/base) * 100.0
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
	
	fmt.Printf(`  Packages seen & %.0f \\`, a.seen)
	fmt.Println()
	fmt.Printf(`  Packages failed to download & %.0f \\`, a.downloadFailed)
	fmt.Println()
	
	fmt.Printf(`  No build failures & %.0f (%f\%%) \\`, a.buildSuccess, percent(a.seen, a.buildSuccess))
	fmt.Println()

	fmt.Println(` \hline`)	

	mean, dev := meanAndDev(a.failedBuildTargetsFailed)
	fmt.Printf(`  Mean failed build targets & %f \\`, mean)
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
	passedBuildFailedTests := float64(len(a.passedBuildFailedTests))
	fmt.Printf(`  No build failures, but test failures & %.0f (%f\%%) \\`, passedBuildFailedTests, percent(a.seen, passedBuildFailedTests))
	fmt.Println()

	fmt.Println(` \hline`)	


	mean, dev := meanAndDev(a.passedBuildFailedTests)
	fmt.Printf(`  Mean failed test targets for passed builds & %f \\`, mean)
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
	
	fmt.Println(`\end{tabular}`)
	fmt.Println(`\end{table}`)	
}

func (a accumulator) emitVersionTable(n int, fail bool) {
	most := a.mostFrequentModules(n)
	failMsg := ""
	if fail {
		failMsg = "fail to"
	}
	fmt.Println(`\begin{table}[ht]`)
	fmt.Printf(`\caption{Most versions per module that %s download}`, failMsg)
	fmt.Println()
	fmt.Println(`\label{table:versions}`)
	fmt.Println(`\begin{tabular}{|l|r|}`)
	fmt.Println(`\hline`)
	for _, data := range most {
		fmt.Printf(" %s & %d \\\\\n", data.name, data.count)
	}
	fmt.Println(`\hline`)
	fmt.Println(`\end{tabular}`)
	fmt.Println(`\end{table}`)

}

func main() {
	var dataDir string
	
	logrus.SetLevel(logrus.WarnLevel)

	flag.StringVar(&dataDir, "datadir", "/home/ingvar/go_data", "Data directory for long-term storage.")

	flag.Parse()
	
	pkgdata.SetStoragePath(dataDir)
	pkgdata.LoadLatest()

	acc, fails := statsRun()

	acc.emitBuildStats()
	fmt.Println()
	acc.emitTestStats()
	fmt.Println()
	acc.emitVersionTable(10, false)

	fails.emitVersionTable(10, true)
}

