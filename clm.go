/**
 * Filename: /Users/bao/code/allhic/allhic/clm.go
 * Path: /Users/bao/code/allhic/allhic
 * Created Date: Monday, January 1st 2018, 5:57:00 pm
 * Author: bao
 *
 * Copyright (c) 2018 Haibao Tang
 */

package allhic

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
)

// CLMFile has the following format:
//
// tig00046211+ tig00063795+       1       53173
// tig00046211+ tig00063795-       1       116050
// tig00046211- tig00063795+       1       71155
// tig00046211- tig00063795-       1       134032
// tig00030676+ tig00077819+       7       136407 87625 87625 106905 102218 169660 169660
// tig00030676+ tig00077819-       7       126178 152952 152952 35680 118923 98367 98367
// tig00030676- tig00077819+       7       118651 91877 91877 209149 125906 146462 146462
// tig00030676- tig00077819-       7       108422 157204 157204 137924 142611 75169 75169
type CLMFile struct {
	Name             string
	Clmfile          string
	Idsfile          string
	Tigs             []TigF
	Tour             Tour
	Signs            []byte
	tigToIdx         map[string]int          // From name of the tig to the idx of the Tigs array
	contacts         map[Pair]Contact        // (tigA, tigB) => {strandedness, nlinks, meanDist}
	orientedContacts map[OrientedPair]GArray // (tigA, tigB, oriA, oriB) => golden array i.e. exponential histogram
}

// Pair contains two contigs in contact
type Pair struct {
	ai int
	bi int
}

// OrientedPair contains two contigs and their orientations
type OrientedPair struct {
	ai int
	bi int
	ao byte
	bo byte
}

// Contact stores how many links between two contigs
type Contact struct {
	strandedness int
	nlinks       int
	meanDist     int
}

// TigF stores the index to activeTigs and size of the tig
type TigF struct {
	Idx      int
	Name     string
	Size     int
	IsActive bool
}

// Tig removes some unnessary entries in the TigF
type Tig struct {
	Idx  int
	Size int
}

// Tour stores a number of tigs along with 2D matrices for evaluation
type Tour struct {
	Tigs []Tig
	M    [][]int
}

// InitCLMFile is the constructor for CLMFile
func InitCLMFile(Clmfile string) *CLMFile {
	p := new(CLMFile)
	p.Name = RemoveExt(path.Base(Clmfile))
	p.Clmfile = Clmfile
	p.Idsfile = RemoveExt(Clmfile) + ".ids"
	p.tigToIdx = make(map[string]int)
	p.contacts = make(map[Pair]Contact)
	p.orientedContacts = make(map[OrientedPair]GArray)

	p.ParseIds()
	p.ParseClm()

	return p
}

// ParseIds parses the idsfile into data stored in CLMFile.
// IDS file has a list of contigs that need to be ordered. 'recover',
// keyword, if available in the third column, is less confident.
// tig00015093     46912
// tig00035238     46779   recover
// tig00030900     119291
func (r *CLMFile) ParseIds() {
	file, _ := os.Open(r.Idsfile)
	log.Noticef("Parse idsfile `%s`", r.Idsfile)
	scanner := bufio.NewScanner(file)
	idx := 0
	for scanner.Scan() {
		words := strings.Fields(scanner.Text())
		tig := words[0]
		size, _ := strconv.Atoi(words[1])
		r.Tigs = append(r.Tigs, TigF{idx, tig, size, true})
		r.tigToIdx[tig] = idx
		idx++
	}
	// fmt.Println(r.Tigs)
}

// rr map orientations to bit ('+' => '-', '-' => '+')
func rr(b byte) byte {
	if b == '-' {
		return '+'
	}
	return '-'
}

// ParseClm parses the clmfile into data stored in CLMFile.
func (r *CLMFile) ParseClm() {
	file, _ := os.Open(r.Clmfile)
	log.Noticef("Parse clmfile `%s`", r.Clmfile)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		row := strings.TrimSpace(scanner.Text())
		words := strings.Split(row, "\t")
		abtig := strings.Split(words[0], " ")
		atig, btig := abtig[0], abtig[1]
		at, ao := atig[:len(atig)-1], atig[len(atig)-1]
		bt, bo := btig[:len(btig)-1], btig[len(btig)-1]

		// Make sure both contigs are in the ids file
		ai, aok := r.tigToIdx[at]
		if !aok {
			continue
		}
		bi, bok := r.tigToIdx[bt]
		if !bok {
			continue
		}

		nlinks, _ := strconv.Atoi(words[1])
		// Convert all distances to int
		var dists []int
		for _, dist := range strings.Split(words[2], " ") {
			d, _ := strconv.Atoi(dist)
			dists = append(dists, d)
		}

		// Store all these info in contacts
		gdists := GoldenArray(dists)
		meanDist := HmeanInt(dists, GRLB, GRUB)
		strandedness := 1
		if ao != bo {
			strandedness = -1
		}
		pair := Pair{ai, bi}
		c := Contact{strandedness, nlinks, meanDist}
		if p, ok := r.contacts[pair]; ok {
			if p.meanDist > meanDist {
				r.contacts[pair] = c
			}
		} else {
			r.contacts[pair] = c
		}
		r.orientedContacts[OrientedPair{ai, bi, ao, bo}] = gdists
		r.orientedContacts[OrientedPair{bi, ai, rr(bo), rr(ao)}] = gdists
	}
	// fmt.Println(r.orientedContacts)
}

// calculateDensities calculated the density of inter-contig links per base.
// Strong contigs are considered to have high level of inter-contig links in the current
// partition.
func (r *CLMFile) calculateDensities() []float64 {
	N := len(r.Tigs)
	densities := make([]int, N)
	for pair, contact := range r.contacts {
		densities[pair.ai] += contact.nlinks
		densities[pair.bi] += contact.nlinks
	}

	logdensities := make([]float64, N)
	for i, tig := range r.Tigs {
		d := float64(densities[i])
		s := float64(min(tig.Size, 500000))
		logdensities[i] = math.Log10(d / s)
	}
	return logdensities
}

// pruneByDensity selects active contigs based on logdensities
func (r *CLMFile) pruneByDensity() {
	logdensities := r.calculateDensities()
	lb, ub := OutlierCutoff(logdensities)
	log.Noticef("Log10(link_densities) ~ [%.5f, %.5f]", lb, ub)
	invalid := 0
	for i, tig := range r.Tigs {
		if logdensities[i] < lb && tig.Size < MINSIZE*10 {
			r.Tigs[i].IsActive = false
			invalid++
		}
	}
	log.Noticef("Inactivated %d tigs with log10_density < %.5f",
		invalid, lb)
}

// pruneBySize selects active contigs based on size
func (r *CLMFile) pruneBySize() {
	invalid := 0
	for i, tig := range r.Tigs {
		if tig.Size < MINSIZE {
			r.Tigs[i].IsActive = false
			invalid++
		}
	}
	log.Noticef("Inactivated %d tigs with size < %d",
		invalid, MINSIZE)
}

// pruneTour test deleting each contig and check the delta_score
func (r *CLMFile) pruneTour() {
	var (
		wg            sync.WaitGroup
		tour, newTour Tour
	)

	for phase := 1; phase < 3; phase++ {
		tour = r.Tour
		tourScore := -tour.Evaluate()
		log.Noticef("Starting score: %.5f", tourScore)
		log10ds := make([]float64, tour.Len()) // Each entry is the log10 of diff

		for i := 0; i < tour.Len(); i++ {
			newTour = tour.Clone().(Tour)
			copy(newTour.Tigs[i:], newTour.Tigs[i+1:]) // Delete element at i
			newTour.Tigs = newTour.Tigs[:newTour.Len()-1]

			wg.Add(1)
			go func(idx int, newTour Tour) {
				defer wg.Done()
				newTourScore := -newTour.Evaluate()
				deltaScore := tourScore - newTourScore
				// log.Noticef("In goroutine %v, newTour = %v, newTourScore = %v, deltaScore = %v",
				// 	idx, newTour.Tigs, newTourScore, deltaScore)
				if deltaScore > 1e-9 {
					log10ds[idx] = math.Log10(deltaScore)
				} else {
					log10ds[idx] = -9.0
				}
			}(i, newTour)
		}
		// Wait for all workers to finish
		wg.Wait()
		fmt.Println(log10ds)

		// Identify outliers
		lb, ub := OutlierCutoff(log10ds)
		log.Noticef("Log10(delta_score) ~ [%.5f, %.5f]", lb, ub)

		invalid := 0
		for i, tig := range tour.Tigs {
			if log10ds[i] < lb {
				r.Tigs[tig.Idx].IsActive = false
				invalid++
			}
		}

		if invalid == 0 {
			break
		} else {
			log.Noticef("Inactivated %d tigs with log10ds < %.5f",
				invalid, lb)
		}

		newTour.Tigs = newTour.Tigs[:0]
		for _, tig := range tour.Tigs {
			if r.Tigs[tig.Idx].IsActive {
				newTour.Tigs = append(newTour.Tigs, tig)
			}
		}
		r.Tour = newTour
		r.reportActive()
	}
}

// Activate selects active contigs in the current partition. This is the setup phase of the
// algorithm, and supports two modes:
// - "de novo": This is useful at the start of a new run where no tours are
//    available. We select the strong contigs that have significant number
//    of links to other contigs in the partition. We build a histogram of
//    link density (# links per bp) and remove the contigs that appear to be
//    outliers. The orientations are derived from the matrix decomposition
//    of the pairwise strandedness matrix O.
// - "hotstart": This is useful when there was a past run, with a given
//    tourfile. In this case, the active contig list and orientations are
//    derived from the last tour in the file.
func (r *CLMFile) Activate(shuffle bool) {
	r.reportActive()
	r.pruneByDensity()
	r.pruneBySize()
	r.reportActive()

	for _, tig := range r.Tigs {
		if tig.IsActive {
			r.Tour.Tigs = append(r.Tour.Tigs, Tig{tig.Idx, tig.Size})
		}
	}

	r.Tour.M = r.M()
	if shuffle {
		r.Tour.Shuffle()
	}
	r.flipAll() // Initialize with the signs of the tigs
}

// reportActive prints number and total length of active contigs
func (r *CLMFile) reportActive() {
	activeCounts := 0
	sumLength := 0
	for _, tig := range r.Tigs {
		if tig.IsActive {
			activeCounts++
			sumLength += tig.Size
		}
	}
	log.Noticef("Active tigs: %d (length=%d)", activeCounts, sumLength)
}

// M yields a contact frequency matrix, where each cell contains how many
// links between i-th and j-th contig
func (r *CLMFile) M() [][]int {
	N := len(r.Tigs)
	P := Make2DSlice(N, N)
	for pair, contact := range r.contacts {
		ai := pair.ai
		bi := pair.bi
		P[ai][bi] = contact.nlinks
		P[bi][ai] = contact.nlinks
	}
	return P
}