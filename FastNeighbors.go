package FastNeighbors

import (
	"log"
	"math"
	"sort"
	"sync"
)

type Interval struct {
	LBound, UBound float64
}

type Point struct {
	ID  int
	Vec []float64
}

type KDLeaf struct {
	BoundingBox []Interval
	Points      []Point
}

type KDTree struct {
	BoundingBox []Interval
	DivisionDim int
	LSubTree    *KDTree
	RSubTree    *KDTree
	LLeaf       *KDLeaf
	RLeaf       *KDLeaf
}

func computeBoundingBox(points []Point) []Interval {
	m := len(points[0].Vec)
	n := len(points)
	boundingBox := make([]Interval, m)
	for i := 0; i < m; i++ {
		boundingBox[i] = Interval{points[0].Vec[i], points[0].Vec[i]}
	}
	for j := 0; j < n; j++ {
		for i := 0; i < m; i++ {
			value := points[j].Vec[i]
			if value < boundingBox[i].LBound {
				boundingBox[i].LBound = value
			} else if value > boundingBox[i].UBound {
				boundingBox[i].UBound = value
			}
		}
	}
	return boundingBox
}

func copyPoints(points []Point) []Point {
	n := len(points)
	copiedPoints := make([]Point, n)
	for j := 0; j < n; j++ {
		copiedPoints[j] = points[j]
	}
	return copiedPoints
}

func NewKDLeaf(points []Point) *KDLeaf {
	return &KDLeaf{
		BoundingBox: computeBoundingBox(points),
		Points:      copyPoints(points),
	}
}

func NewKDTree(points []Point, numPointsPerLeaf int) *KDTree {
	// get dims of data
	n := len(points)
	if n == 0 {
		log.Fatalln("no input points passed to NewKDTree")
	}
	m := len(points[0].Vec)
	if m == 0 {
		log.Fatalln("input point of 0 dims is not allowed")
	}

	// check data
	for j := 1; j < n; j++ {
		if len(points[j].Vec) != m {
			log.Fatalln("dims of input points don't match")
		}
	}

	// check numPointsPerLeaf
	if numPointsPerLeaf < 1 {
		log.Fatalln("numPointsPerLeaf is not allowed to be < 1")
	}

	// return a KD tree with all these points in one leaf
	result := &KDTree{
		BoundingBox: computeBoundingBox(points),
		DivisionDim: -1,
		LSubTree:    nil,
		RSubTree:    nil,
		LLeaf:       NewKDLeaf(points),
		RLeaf:       nil,
	}
	result.split(numPointsPerLeaf)
	return result
}

func (kdtree *KDTree) split(numPointsPerLeaf int) {
	if kdtree.DivisionDim != -1 {
		log.Fatalln("a KDTree is not allowed to split twice")
	}

	// find the dim with the largest variance
	m := len(kdtree.BoundingBox)
	points := kdtree.LLeaf.Points
	n := len(points)
	means := make([]float64, m)
	vars := make([]float64, m)
	for i := 0; i < m; i++ {
		means[i] = 0.0
		vars[i] = 0.0
	}
	for j := 0; j < n; j++ {
		point := points[j]
		for i := 0; i < m; i++ {
			means[i] += point.Vec[i]
			vars[i] += point.Vec[i] * point.Vec[i]
		}
	}
	for i := 0; i < m; i++ {
		means[i] /= float64(n)
		vars[i] -= float64(n) * means[i] * means[i]
	}

	dimWithMaxVar := 0
	for i := 1; i < m; i++ {
		if vars[i] > vars[dimWithMaxVar] {
			dimWithMaxVar = i
		}
	}

	// split along dim with the largest variance
	kdtree.DivisionDim = dimWithMaxVar
	sort.Slice(points, func(i, j int) bool {
		return points[i].Vec[dimWithMaxVar] < points[j].Vec[dimWithMaxVar]
	})
	leftPoints := points[0 : n/2]
	rightPoints := points[n/2 : n]
	kdtree.LLeaf = nil

	// for each part, split it if it is too large
	var waitGroup sync.WaitGroup
	waitGroup.Add(1)

	go func(wg *sync.WaitGroup) {
		if len(rightPoints) > numPointsPerLeaf {
			kdtree.RSubTree = NewKDTree(rightPoints, numPointsPerLeaf)
		} else {
			kdtree.RLeaf = NewKDLeaf(rightPoints)
		}
		wg.Done()
	}(&waitGroup)

	if len(leftPoints) > numPointsPerLeaf {
		kdtree.LSubTree = NewKDTree(leftPoints, numPointsPerLeaf)
	} else {
		kdtree.LLeaf = NewKDLeaf(leftPoints)
	}
	waitGroup.Wait()
}

func findNearestPointInBoundingBox(center []float64, boundingBox []Interval,
) []float64 {
	m := len(boundingBox)
	result := make([]float64, m)
	for i := 0; i < m; i++ {
		if center[i] < boundingBox[i].LBound {
			result[i] = boundingBox[i].LBound
		} else if center[i] > boundingBox[i].UBound {
			result[i] = boundingBox[i].UBound
		} else {
			result[i] = center[i]
		}
	}
	return result
}

func computeDistanceToBoundingBox(center []float64, boundingBox []Interval,
) float64 {
	nearestPoint := findNearestPointInBoundingBox(center, boundingBox)
	return computeDistanceToPoint(center, nearestPoint)
}

func computeDistanceToPoint(center []float64, point []float64) float64 {
	result := 0.0
	for i, value := range center {
		dOfDim := value - point[i]
		result += dOfDim * dOfDim
	}
	result = math.Sqrt(result)
	return result
}

func (kdtree *KDTree) FindNeighbors(center []float64, radius float64) []Point {
	m := len(kdtree.BoundingBox)
	if len(center) != m {
		log.Fatalln("Dims of center does not match dims of kd-tree")
	}

	result := []Point{}
	distance := computeDistanceToBoundingBox(center, kdtree.BoundingBox)
	if distance > radius {
		return result
	}

	if kdtree.LSubTree != nil {
		childrenResult := kdtree.LSubTree.FindNeighbors(center, radius)
		result = append(result, childrenResult...)
	} else if kdtree.LLeaf != nil {
		childrenResult := kdtree.LLeaf.findNeighbors(center, radius)
		result = append(result, childrenResult...)
	}
	if kdtree.RSubTree != nil {
		childrenResult := kdtree.RSubTree.FindNeighbors(center, radius)
		result = append(result, childrenResult...)
	} else if kdtree.RLeaf != nil {
		childrenResult := kdtree.RLeaf.findNeighbors(center, radius)
		result = append(result, childrenResult...)
	}

	return result
}

func (kdleaf *KDLeaf) findNeighbors(center []float64, radius float64) []Point {
	m := len(kdleaf.BoundingBox)
	if len(center) != m {
		log.Fatalln("Dims of center does not match dims of kd-leaf")
	}

	result := []Point{}
	distance := computeDistanceToBoundingBox(center, kdleaf.BoundingBox)
	if distance > radius {
		return result
	}

	n := len(kdleaf.Points)
	for j := 0; j < n; j++ {
		distance = computeDistanceToPoint(center, kdleaf.Points[j].Vec)
		if distance <= radius {
			result = append(result, kdleaf.Points[j])
		}
	}

	return result
}