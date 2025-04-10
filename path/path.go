package path

import (
	"math"
	"slices"
)

type Node struct {
	x, y     int
	children []*Node
}

func getOrNil(nodes [][]*Node, x, y int) *Node {
	if y < 0 || y >= len(nodes) || x < 0 || x >= len(nodes[y]) {
		return nil
	}
	return nodes[y][x]
}

func getOrNilMaze(maze [][]bool, y, x int) (Node2, bool) {
	if y < 0 || y >= len(maze) || x < 0 || x >= len(maze[y]) || maze[y][x] == false {
		return Node2{}, false
	}
	return Node2{x, y}, true
}

func makeTreeFromMaze(maze [][]bool, startX, startY int) *Node {
	nodes := make([][]*Node, len(maze))
	for y, row := range maze {
		nodes[y] = make([]*Node, len(maze[y]))

		for x, wall := range row {
			if !wall {
				nodes[y][x] = &Node{x: x, y: y}
			}
		}
	}
	for y := range nodes {
		for x, node := range nodes[y] {
			if node != nil {
				if child := getOrNil(nodes, y-1, x-1); child != nil {
					node.children = append(node.children, child)
				}
				if child := getOrNil(nodes, y-1, x); child != nil {
					node.children = append(node.children, child)
				}
				if child := getOrNil(nodes, y-1, x+1); child != nil {
					node.children = append(node.children, child)
				}
				if child := getOrNil(nodes, y, x-1); child != nil {
					node.children = append(node.children, child)
				}
				if child := getOrNil(nodes, y, x+1); child != nil {
					node.children = append(node.children, child)
				}
				if child := getOrNil(nodes, y+1, x-1); child != nil {
					node.children = append(node.children, child)
				}
				if child := getOrNil(nodes, y+1, x); child != nil {
					node.children = append(node.children, child)
				}
				if child := getOrNil(nodes, y+1, x+1); child != nil {
					node.children = append(node.children, child)
				}
			}
		}
	}

	return nodes[startY][startX]
}

func getChildNodes(maze [][]bool, node Node2) []Node2 {
	x := node.X
	y := node.Y
	nodes := make([]Node2, 0)

	//if child, ok := getOrNilMaze(maze, y-1, x-1); ok {
	//	nodes = append(nodes, child)
	//}
	if child, ok := getOrNilMaze(maze, y-1, x); ok {
		nodes = append(nodes, child)
	}
	//if child, ok := getOrNilMaze(maze, y-1, x+1); ok {
	//	nodes = append(nodes, child)
	//}
	if child, ok := getOrNilMaze(maze, y, x-1); ok {
		nodes = append(nodes, child)
	}
	if child, ok := getOrNilMaze(maze, y, x+1); ok {
		nodes = append(nodes, child)
	}
	//if child, ok := getOrNilMaze(maze, y+1, x-1); ok {
	//	nodes = append(nodes, child)
	//}
	if child, ok := getOrNilMaze(maze, y+1, x); ok {
		nodes = append(nodes, child)
	}
	//if child, ok := getOrNilMaze(maze, y+1, x+1); ok {
	//	nodes = append(nodes, child)
	//}

	return nodes
}

//func BruteForce(maze [][]bool, startX, startY, endX, endY int) {
//	tree := makeTreeFromMaze(maze, startX, startY)
//
//	visited := make([][]bool, len(maze))
//	for y := range maze {
//		visited[y] = make([]bool, len(maze[y]))
//	}
//
//	queue := make([]*Node, 0)
//	for len(queue) > 0 && queue[0] != nil {
//		node := queue[0]
//
//		if node.x == endX && node.y == endY {
//
//		}
//	}
//}

func BruteForceFind(node *Node, visited *[][]bool, endX, endY int) {
	queue := make([]*Node, 0)
	for len(queue) > 0 && queue[0] != nil {
		node := queue[0]

		if node.x == endX && node.y == endY {

		}
	}
}

type DijkstraNode struct {
	*Node

	distance int
	pathVia  *Node
}

func InsertOrdered(slice []DijkstraNode, el DijkstraNode) []DijkstraNode {
	index := len(slice) - 1
	for i, node := range slice {
		if node.distance > el.distance {
			index = i
		}
	}
	return slices.Insert(slice, index, el)
}

func InsertOrdered1(slice []*DNode, el *DNode) []*DNode {
	index := len(slice)
	for i, node := range slice {
		if float64(node.distance)+node.distanceRemaining > float64(el.distance)+el.distanceRemaining {
			index = i
		}
	}
	return slices.Insert(slice, index, el)
}

func UpdateOrder1(slice []*DNode, el *DNode) []*DNode {
	oldIndex := slices.Index(slice, el)
	slice = slices.Delete(slice, oldIndex, oldIndex)
	return InsertOrdered1(slice, el)
}

type Node2 struct {
	X, Y int
}
type DNode struct {
	Node2

	distance          int
	pathVia           *DNode
	distanceRemaining float64
}

func Dijkstra(maze [][]bool, startX, startY, endX, endY int) []Node2 {
	return dijkstraOrAStar(false, maze, startX, startY, endX, endY)
}

func Astart(maze [][]bool, startX, startY, endX, endY int) []Node2 {
	return dijkstraOrAStar(true, maze, startX, startY, endX, endY)
}

func dijkstraOrAStar(astar bool, maze [][]bool, startX, startY, endX, endY int) []Node2 {
	nodes := make([][]*DNode, len(maze))
	for y := range nodes {
		nodes[y] = make([]*DNode, len(maze[y]))
	}

	startNode := &DNode{Node2: Node2{startX, startY}, distance: 0, pathVia: nil}
	nodes[startY][startX] = startNode

	finished := make([]*DNode, 0)

	pqueue := make([]*DNode, 0)
	pqueue = append(pqueue, startNode)

	for {
		top := pqueue[0]
		pqueue = pqueue[1:]
		if top.X == endX && top.Y == endY {
			// todo
			pqueue = append(pqueue, top)

			solution := make([]Node2, 0)

			currNode := top
			for currNode != nil {
				solution = append(solution, currNode.Node2)
				currNode = currNode.pathVia
			}
			slices.Reverse(solution)
			return solution
		}

		children := getChildNodes(maze, top.Node2)
		for _, child := range children {
			childNode := nodes[child.Y][child.X]
			if childNode != nil {
				if childNode == top.pathVia {
					continue
				}
				newDistance := childNode.distance + AbsInt(child.X-top.X) + AbsInt(child.Y-top.Y)
				if newDistance >= childNode.distance {
					continue
				}

				childNode.distance = newDistance
				childNode.pathVia = top
				pqueue = UpdateOrder1(pqueue, childNode)
			} else {
				childNode = &DNode{Node2: child}
				childNode.distance = AbsInt(child.X-top.X) + AbsInt(child.Y-top.Y)
				childNode.pathVia = top
				if astar {
					distX := float64(child.X - endX)
					distY := float64(child.Y - endY)
					dist := math.Sqrt(distX*distX + distY*distY)

					childNode.distanceRemaining = dist
				}

				nodes[child.Y][child.X] = childNode
				pqueue = InsertOrdered1(pqueue, childNode)
			}
		}

		finished = append(finished, top)
	}
}

func AbsInt(a int) int {
	if a < 0 {
		return -a
	}
	return a
}
