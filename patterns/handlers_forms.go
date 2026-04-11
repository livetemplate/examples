package main

import (
	"net/http"
	"slices"

	"github.com/livetemplate/livetemplate"
)

// --- Pattern #1: Click To Edit ---

type ClickToEditController struct{}

func (c *ClickToEditController) Edit(state ClickToEditState, ctx *livetemplate.Context) (ClickToEditState, error) {
	state.Editing = true
	return state, nil
}

func (c *ClickToEditController) Save(state ClickToEditState, ctx *livetemplate.Context) (ClickToEditState, error) {
	state.FirstName = ctx.GetString("firstName")
	state.LastName = ctx.GetString("lastName")
	state.Email = ctx.GetString("email")
	state.Editing = false
	return state, nil
}

func (c *ClickToEditController) Cancel(state ClickToEditState, ctx *livetemplate.Context) (ClickToEditState, error) {
	state.Editing = false
	return state, nil
}

func clickToEditHandler(baseOpts []livetemplate.Option) http.Handler {
	opts := append(slices.Clone(baseOpts),
		livetemplate.WithParseFiles("templates/layout.tmpl", "templates/forms/click-to-edit.tmpl"),
	)
	tmpl := livetemplate.Must(livetemplate.New("layout", opts...))
	return tmpl.Handle(&ClickToEditController{}, livetemplate.AsState(&ClickToEditState{
		Title:     "Click To Edit",
		Category:  "Forms & Editing",
		FirstName: "John",
		LastName:  "Doe",
		Email:     "john@example.com",
	}))
}

// --- Pattern #2: Edit Row ---

type EditRowController struct{}

func (c *EditRowController) Edit(state EditRowState, ctx *livetemplate.Context) (EditRowState, error) {
	state.EditingID = ctx.GetString("id")
	return state, nil
}

func (c *EditRowController) Save(state EditRowState, ctx *livetemplate.Context) (EditRowState, error) {
	id := ctx.GetString("id")
	for i, contact := range state.Contacts {
		if contact.ID == id {
			state.Contacts[i].Name = ctx.GetString("name")
			state.Contacts[i].Email = ctx.GetString("email")
			break
		}
	}
	state.EditingID = ""
	return state, nil
}

func (c *EditRowController) Cancel(state EditRowState, ctx *livetemplate.Context) (EditRowState, error) {
	state.EditingID = ""
	return state, nil
}

func editRowHandler(baseOpts []livetemplate.Option) http.Handler {
	opts := append(slices.Clone(baseOpts),
		livetemplate.WithParseFiles("templates/layout.tmpl", "templates/forms/edit-row.tmpl"),
	)
	tmpl := livetemplate.Must(livetemplate.New("layout", opts...))
	return tmpl.Handle(&EditRowController{}, livetemplate.AsState(&EditRowState{
		Title:    "Edit Row",
		Category: "Forms & Editing",
		Contacts: sampleContacts(),
	}))
}

// --- Pattern #3: Inline Validation ---

type InlineValidationController struct{}

func (c *InlineValidationController) Change(state InlineValidationState, ctx *livetemplate.Context) (InlineValidationState, error) {
	if ctx.Has("email") {
		state.Email = ctx.GetString("email")
	}
	if ctx.Has("username") {
		state.Username = ctx.GetString("username")
	}
	_ = ctx.ValidateForm()
	return state, nil
}

func (c *InlineValidationController) Submit(state InlineValidationState, ctx *livetemplate.Context) (InlineValidationState, error) {
	if err := ctx.ValidateForm(); err != nil {
		return state, err
	}
	state.Saved = true
	return state, nil
}

func inlineValidationHandler(baseOpts []livetemplate.Option) http.Handler {
	opts := append(slices.Clone(baseOpts),
		livetemplate.WithParseFiles("templates/layout.tmpl", "templates/forms/inline-validation.tmpl"),
	)
	tmpl := livetemplate.Must(livetemplate.New("layout", opts...))
	return tmpl.Handle(&InlineValidationController{}, livetemplate.AsState(&InlineValidationState{
		Title:    "Inline Validation",
		Category: "Forms & Editing",
	}))
}

// --- Pattern #4: Bulk Update ---

type BulkUpdateController struct{}

func (c *BulkUpdateController) BulkUpdate(state BulkUpdateState, ctx *livetemplate.Context) (BulkUpdateState, error) {
	for i, user := range state.Users {
		state.Users[i].Active = ctx.GetBool("active-" + user.ID)
	}
	return state, nil
}

func bulkUpdateHandler(baseOpts []livetemplate.Option) http.Handler {
	opts := append(slices.Clone(baseOpts),
		livetemplate.WithParseFiles("templates/layout.tmpl", "templates/forms/bulk-update.tmpl"),
	)
	tmpl := livetemplate.Must(livetemplate.New("layout", opts...))
	return tmpl.Handle(&BulkUpdateController{}, livetemplate.AsState(&BulkUpdateState{
		Title:    "Bulk Update",
		Category: "Forms & Editing",
		Users:    sampleUsers(),
	}))
}

// --- Pattern #5: Reset User Input ---

type ResetInputController struct{}

func (c *ResetInputController) Submit(state ResetInputState, ctx *livetemplate.Context) (ResetInputState, error) {
	msg := ctx.GetString("message")
	if msg != "" {
		state.Messages = append(state.Messages, msg)
	}
	return state, nil
}

func resetInputHandler(baseOpts []livetemplate.Option) http.Handler {
	opts := append(slices.Clone(baseOpts),
		livetemplate.WithParseFiles("templates/layout.tmpl", "templates/forms/reset-input.tmpl"),
	)
	tmpl := livetemplate.Must(livetemplate.New("layout", opts...))
	return tmpl.Handle(&ResetInputController{}, livetemplate.AsState(&ResetInputState{
		Title:    "Reset User Input",
		Category: "Forms & Editing",
	}))
}

// --- Pattern #6: File Upload ---

type FileUploadController struct{}

func (c *FileUploadController) Submit(state FileUploadState, ctx *livetemplate.Context) (FileUploadState, error) {
	for _, name := range []string{"document", "chunked-doc"} {
		if ctx.HasUploads(name) {
			entries := ctx.GetCompletedUploads(name)
			if len(entries) > 0 {
				state.UploadName = entries[0].ClientName
				state.Uploaded = true
				return state, nil
			}
		}
	}
	return state, nil
}

func fileUploadHandler(baseOpts []livetemplate.Option) http.Handler {
	opts := append(slices.Clone(baseOpts),
		livetemplate.WithParseFiles("templates/layout.tmpl", "templates/forms/file-upload.tmpl"),
		livetemplate.WithUpload("chunked-doc", livetemplate.UploadConfig{
			MaxFileSize: 10 << 20, // 10 MB
			MaxEntries:  1,
		}),
	)
	tmpl := livetemplate.Must(livetemplate.New("layout", opts...))
	return tmpl.Handle(&FileUploadController{}, livetemplate.AsState(&FileUploadState{
		Title:    "File Upload",
		Category: "Forms & Editing",
	}))
}

// --- Pattern #7: Preserving File Inputs ---

type PreserveInputsController struct{}

func (c *PreserveInputsController) Submit(state PreserveInputsState, ctx *livetemplate.Context) (PreserveInputsState, error) {
	state.Name = ctx.GetString("name")
	state.Description = ctx.GetString("description")
	if err := ctx.ValidateForm(); err != nil {
		return state, err
	}
	return state, nil
}

func preserveInputsHandler(baseOpts []livetemplate.Option) http.Handler {
	opts := append(slices.Clone(baseOpts),
		livetemplate.WithParseFiles("templates/layout.tmpl", "templates/forms/preserve-inputs.tmpl"),
	)
	tmpl := livetemplate.Must(livetemplate.New("layout", opts...))
	return tmpl.Handle(&PreserveInputsController{}, livetemplate.AsState(&PreserveInputsState{
		Title:    "Preserving File Inputs",
		Category: "Forms & Editing",
	}))
}
