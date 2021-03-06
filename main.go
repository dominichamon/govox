package main

import (
	"errors"
	"log"
	"math/rand"
	"runtime"
	"strings"

	"github.com/go-gl/gl/v3.3-core/gl"
	"github.com/go-gl/glfw/v3.2/glfw"
	"github.com/go-gl/mathgl/mgl32"
)

const (
	width  = 1600
	height = 900

	size = 50
)

func init() {
	runtime.LockOSThread()
}

type block struct {
	active bool
	color  mgl32.Vec4
}

func main() {
	if err := glfw.Init(); err != nil {
		log.Fatal(err)
	}
	defer glfw.Terminate()

	glfw.WindowHint(glfw.Resizable, glfw.False)
	glfw.WindowHint(glfw.ContextVersionMajor, 3)
	glfw.WindowHint(glfw.ContextVersionMinor, 3)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)

	window, err := glfw.CreateWindow(width, height, "govox", nil, nil)
	if err != nil {
		log.Fatal(err)
	}
	window.MakeContextCurrent()

	if err := gl.Init(); err != nil {
		log.Fatal(err)
	}

	log.Printf("OpenGL version %s", gl.GoStr(gl.GetString(gl.VERSION)))

	// Set up the program
	p, err := newProgram(vertexShader, fragmentShader)
	if err != nil {
		log.Fatal(err)
	}
	gl.UseProgram(p)

	proj := mgl32.Perspective(mgl32.DegToRad(45.0), float32(width)/height, 0.1, 1000.0)
	projUni := gl.GetUniformLocation(p, gl.Str("projection\x00"))
	gl.UniformMatrix4fv(projUni, 1, false, &proj[0])

	cam := mgl32.LookAtV(mgl32.Vec3{size * 1.8, size * 1.5, size * 2}, mgl32.Vec3{0, 0, 0}, mgl32.Vec3{0, 1, 0})
	camUni := gl.GetUniformLocation(p, gl.Str("camera\x00"))
	gl.UniformMatrix4fv(camUni, 1, false, &cam[0])

	model := mgl32.Ident4()
	modelUni := gl.GetUniformLocation(p, gl.Str("model\x00"))
	gl.UniformMatrix4fv(modelUni, 1, false, &model[0])

	col := mgl32.Vec4{0, 0, 0, 1}
	colUni := gl.GetUniformLocation(p, gl.Str("col\x00"))
	gl.Uniform4fv(colUni, 1, &col[0])

	gl.BindFragDataLocation(p, 0, gl.Str("outputColor\x00"))

	// Vertex data
	var vao uint32
	gl.GenVertexArrays(1, &vao)
	gl.BindVertexArray(vao)

	var vbo uint32
	gl.GenBuffers(1, &vbo)
	gl.BindBuffer(gl.ARRAY_BUFFER, vbo)
	gl.BufferData(gl.ARRAY_BUFFER, len(cubeVerts)*4, gl.Ptr(cubeVerts), gl.STATIC_DRAW)

	vertAttrib := uint32(gl.GetAttribLocation(p, gl.Str("vert\x00")))
	gl.EnableVertexAttribArray(vertAttrib)
	gl.VertexAttribPointer(vertAttrib, 3, gl.FLOAT, false, 3*4, gl.PtrOffset(0))

	// globals
	gl.Enable(gl.DEPTH_TEST)
	gl.DepthFunc(gl.LESS)
	gl.ClearColor(0.8, 0.8, 1.0, 1.0)

	// initialize blocks
	blocks := make([][][]block, size)
	for i := 0; i < size; i++ {
		blocks[i] = make([][]block, size)
		for j := 0; j < size; j++ {
			blocks[i][j] = make([]block, size)
			for k := 0; k < size; k++ {
				blocks[i][j][k] = block{
					active: (rand.Float32() < 0.1),
					color: mgl32.Vec4{
						rand.Float32(),
						rand.Float32(),
						rand.Float32(),
						1.0,
					},
				}
			}
		}
	}

	var roty, rotx float32

	for !window.ShouldClose() {
		// check inputs
		if window.GetKey(glfw.KeyLeft) == glfw.Press {
			roty -= 0.05
		}

		if window.GetKey(glfw.KeyRight) == glfw.Press {
			roty += 0.05
		}

		if window.GetKey(glfw.KeyUp) == glfw.Press {
			rotx -= 0.05
		}

		if window.GetKey(glfw.KeyDown) == glfw.Press {
			rotx += 0.05
		}

		gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

		for i := 0; i < size; i++ {
			for j := 0; j < size; j++ {
				for k := 0; k < size; k++ {
					b := blocks[i][j][k]
					if !b.active {
						continue
					}

					gl.Uniform4fv(colUni, 1, &b.color[0])

					fi := float32(i) - size/2
					fj := float32(j) - size/2
					fk := float32(k) - size/2

					model = mgl32.HomogRotate3DY(roty)
					model = model.Mul4(mgl32.HomogRotate3DX(rotx))
					model = model.Mul4(mgl32.Translate3D(fi, fj, fk))
					model = model.Mul4(mgl32.Scale3D(0.5, 0.5, 0.5))

					gl.UniformMatrix4fv(modelUni, 1, false, &model[0])
					gl.BindVertexArray(vao)
					gl.DrawArrays(gl.TRIANGLES, 0, 6*2*3)
				}
			}
		}

		window.SwapBuffers()
		glfw.PollEvents()
	}
}

func newProgram(vSource, fSource string) (uint32, error) {
	vShader, err := compileShader(vSource, gl.VERTEX_SHADER)
	if err != nil {
		return 0, err
	}

	fShader, err := compileShader(fSource, gl.FRAGMENT_SHADER)
	if err != nil {
		return 0, err
	}

	p := gl.CreateProgram()

	gl.AttachShader(p, vShader)
	gl.AttachShader(p, fShader)
	gl.LinkProgram(p)

	var status int32
	gl.GetProgramiv(p, gl.LINK_STATUS, &status)
	if status == gl.FALSE {
		var ll int32
		gl.GetProgramiv(p, gl.INFO_LOG_LENGTH, &ll)

		l := strings.Repeat("\x00", int(ll+1))
		gl.GetProgramInfoLog(p, ll, nil, gl.Str(l))

		return 0, errors.New(l)
	}

	gl.DeleteShader(vShader)
	gl.DeleteShader(fShader)

	return p, nil
}

func compileShader(s string, t uint32) (uint32, error) {
	shader := gl.CreateShader(t)

	cs, free := gl.Strs(s)
	gl.ShaderSource(shader, 1, cs, nil)
	free()
	gl.CompileShader(shader)

	var status int32
	gl.GetShaderiv(shader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var ll int32
		gl.GetShaderiv(shader, gl.INFO_LOG_LENGTH, &ll)

		l := strings.Repeat("\x00", int(ll+1))
		gl.GetShaderInfoLog(shader, ll, nil, gl.Str(l))

		return 0, errors.New(l)
	}

	return shader, nil
}
