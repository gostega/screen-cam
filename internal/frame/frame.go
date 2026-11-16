// Package frame turns a captured BGRA screen region into the fixed-size I420
// (YUV 4:2:0) picture the virtual camera carries. The output size never
// changes (ADR-0001); a viewport of any shape is scaled to fit inside it,
// with black bars where the shapes differ.
package frame

// Fit returns where a src-sized picture lands inside dst when scaled to fit
// while keeping its aspect ratio: offset (x, y) and scaled size (w, h).
func Fit(srcW, srcH, dstW, dstH int) (x, y, w, h int) {
	// Compare srcW/srcH with dstW/dstH without floating point.
	if srcW*dstH >= srcH*dstW { // wider than the output: full width
		w, h = dstW, srcH*dstW/srcW
	} else { // taller: full height
		w, h = srcW*dstH/srcH, dstH
	}
	w, h = w&^1, h&^1 // even, so chroma rows and columns line up
	return (dstW - w) / 2 &^ 1, (dstH - h) / 2 &^ 1, w, h
}

// I420 is one output picture: a full-size luma plane then quarter-size U and V.
type I420 struct {
	W, H int
	Buf  []byte
}

// NewI420 allocates a w×h picture. w and h must be even.
func NewI420(w, h int) *I420 { return &I420{W: w, H: h, Buf: make([]byte, w*h*3/2)} }

func (f *I420) planes() (y, u, v []byte) {
	n := f.W * f.H
	return f.Buf[:n], f.Buf[n : n+n/4], f.Buf[n+n/4:]
}

// Fill paints the whole picture one colour.
func (f *I420) Fill(r, g, b byte) {
	yy, uu, vv := toYUV(int(r), int(g), int(b))
	y, u, v := f.planes()
	for i := range y {
		y[i] = yy
	}
	for i := range u {
		u[i], v[i] = uu, vv
	}
}

// Scaler maps a fixed source size into a fixed output, precomputing the
// nearest-neighbour lookups so each frame is only table reads.
type Scaler struct {
	srcW, srcH int
	out        *I420
	x, y, w, h int   // where the picture lands in the output
	colX, rowY []int // output column/row -> source column/row
}

// NewScaler prepares scaling from srcW×srcH into out.
func NewScaler(srcW, srcH int, out *I420) *Scaler {
	s := &Scaler{srcW: srcW, srcH: srcH, out: out}
	s.x, s.y, s.w, s.h = Fit(srcW, srcH, out.W, out.H)
	s.colX = make([]int, s.w)
	for i := range s.colX {
		s.colX[i] = i * srcW / s.w
	}
	s.rowY = make([]int, s.h)
	for i := range s.rowY {
		s.rowY[i] = i * srcH / s.h
	}
	return s
}

// Matches reports whether this scaler was built for a srcW×srcH source.
func (s *Scaler) Matches(srcW, srcH int) bool { return s.srcW == srcW && s.srcH == srcH }

// Render draws src (BGRA, 4 bytes a pixel, rows stride bytes apart) into the
// output, black around it.
func (s *Scaler) Render(src []byte, stride int) {
	s.out.Fill(0, 0, 0)
	Y, U, V := s.out.planes()
	W := s.out.W
	for oy := 0; oy < s.h; oy++ {
		row := src[s.rowY[oy]*stride:]
		dy := s.y + oy
		yline := Y[dy*W+s.x:]
		chroma := dy%2 == 0
		for ox := 0; ox < s.w; ox++ {
			p := row[s.colX[ox]*4:]
			r, g, b := int(p[2]), int(p[1]), int(p[0])
			yline[ox] = byte((66*r+129*g+25*b+128)>>8 + 16)
			if chroma && ox%2 == 0 {
				c := (dy/2)*(W/2) + (s.x+ox)/2
				U[c] = byte((-38*r-74*g+112*b+128)>>8 + 128)
				V[c] = byte((112*r-94*g-18*b+128)>>8 + 128)
			}
		}
	}
}

// toYUV is BT.601 limited range, the format browsers assume for YU12 webcams.
func toYUV(r, g, b int) (y, u, v byte) {
	return byte((66*r+129*g+25*b+128)>>8 + 16),
		byte((-38*r-74*g+112*b+128)>>8 + 128),
		byte((112*r-94*g-18*b+128)>>8 + 128)
}
