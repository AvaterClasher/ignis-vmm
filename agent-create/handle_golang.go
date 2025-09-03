package main

import (
	"bytes"
	"net/http"
	"os"
	"os/exec"

	"github.com/labstack/echo/v4"
)

func copy(src string, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	err = os.WriteFile(dst, data, 0644)
	return err
}

func golangHandler(c echo.Context, req *runReq) error {
	err := copy("/tmp/"+req.ID, "/tmp/"+req.ID+".go")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, runCRes{
			Message: "Failed to copy file",
			Error:   err.Error(),
		})
	}

	var compileStdOut, compileStdErr bytes.Buffer
	compileCmd := exec.Command("go", "build", "-o", "/tmp/"+req.ID+".out", "/tmp/"+req.ID+".go")
	compileCmd.Stdout = &compileStdOut
	compileCmd.Stderr = &compileStdErr
	err = compileCmd.Run()

	if err != nil {
		return c.JSON(http.StatusBadRequest, runCRes{
			Message: "Failed to compile",
			Error:   err.Error(),
			Stdout:  compileStdOut.String(),
			Stderr:  compileStdErr.String(),
		})
	}

	return execCmd(c, "/tmp/"+req.ID+".out")
}
