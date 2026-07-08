package renderer

func (r *Renderer) allocateUniformBuffer(size int) (uint32, []byte) {
	if r == nil {
		return 0, nil
	}

	offset := r.uniformOffset
	if offset%worldUniformAlign != 0 {
		offset = (offset + worldUniformAlign - 1) / worldUniformAlign * worldUniformAlign
	}

	if offset+uint32(size) > worldUniformAlign*worldUniformMaxDraws {
		// Out of space! Just return 0 to prevent crash, though visuals will be wrong.
		if len(r.uniformDataScratch) < size {
			r.uniformDataScratch = make([]byte, size)
		}
		return 0, r.uniformDataScratch[:size]
	}

	neededCap := int(offset) + size
	if cap(r.uniformDataScratch) < neededCap {
		newCap := cap(r.uniformDataScratch) * 2
		if newCap < neededCap {
			newCap = neededCap
		}
		if newCap < worldUniformAlign*worldUniformMaxDraws {
			newCap = worldUniformAlign * worldUniformMaxDraws
		}
		newData := make([]byte, newCap)
		copy(newData, r.uniformDataScratch)
		r.uniformDataScratch = newData
	}

	r.uniformDataScratch = r.uniformDataScratch[:neededCap]
	r.uniformOffset = offset + uint32(size)

	return offset, r.uniformDataScratch[offset : offset+uint32(size)]
}

func (r *Renderer) resetUniformBuffer() {
	if r != nil {
		r.uniformOffset = 0
	}
}
