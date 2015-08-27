package redux

import "os"

// An Output is the output of a .do scripts, either through stdout or $3 (Arg3)
// If the .do script invocation is equivalent to the sh command,
//
//	  sh target.ext.do target.ext target tmp0 > tmp1
//
// tmp0 and tmp1 would be outputs.
type Output struct {
	*os.File
	IsArg3 bool
}

func (out *Output) Size() (size int64, err error) {
	var finfo os.FileInfo

	if out.IsArg3 {
		// f.Stat() doesn't work for the file on $3 since it was written to by a different process.
		finfo, err = os.Stat(out.Name())
	} else {
		finfo, err = out.Stat()
	}

	if err == nil {
		size = finfo.Size()
	}
	return
}
