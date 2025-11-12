# Avatar Upload Example

A simple example demonstrating LiveTemplate's file upload feature with avatar upload functionality.

## Features

- 📸 **Image Upload**: Upload JPEG, PNG, or GIF avatars
- 📊 **Real-time Progress**: WebSocket chunked upload with live progress tracking
- ✅ **Validation**: Automatic file type and size validation (5MB limit)
- 🎨 **Beautiful UI**: Gradient design with smooth animations
- 🔄 **Live Updates**: Profile updates instantly without page reload

## What This Example Demonstrates

### Upload Configuration
```go
func (s *ProfileStore) AllowUploads() map[string]livetemplate.UploadConfig {
    return map[string]livetemplate.UploadConfig{
        "avatar": {
            Accept:      []string{"image/jpeg", "image/png", "image/gif"},
            MaxFileSize: 5 * 1024 * 1024, // 5MB
            MaxEntries:  1,                // Single file
            AutoUpload:  false,            // Manual upload on form submit
            ChunkSize:   256 * 1024,       // 256KB chunks
        },
    }
}
```

### Upload Processing
```go
func (s *ProfileStore) ConsumeUpload(ctx context.Context, name string, entries []*livetemplate.UploadEntry) error {
    for _, entry := range entries {
        // Move from temp to permanent location
        permanentPath := filepath.Join("uploads", fmt.Sprintf("avatar-%s%s", entry.ID, ext))
        os.Rename(entry.TempPath, permanentPath)

        // Update store with new avatar
        s.AvatarPath = permanentPath
        s.AvatarURL = "/" + permanentPath
    }
    return nil
}
```

### Template Helpers
```html
<input type="file" lvt-upload="avatar" accept="image/jpeg,image/png,image/gif">

<!-- Show upload progress -->
{{range .lvt.Uploads "avatar"}}
    <div class="upload-entry">
        <span>{{.ClientName}} - {{.Progress}}%</span>
        <progress value="{{.Progress}}" max="100"></progress>
        {{if .Error}}<span class="error">{{.Error}}</span>{{end}}
    </div>
{{end}}
```

## Running the Example

### 1. Install Dependencies

```bash
cd /Users/adnaan/code/livetemplate/examples/avatar-upload
go mod download
```

### 2. Run the Server

```bash
go run main.go
```

The server will start at http://localhost:8080

### 3. Try It Out

1. Open http://localhost:8080 in your browser
2. Click "Choose File" and select an image (JPEG, PNG, or GIF)
3. Click "Save Profile"
4. Watch the real-time progress bar as your file uploads
5. See your avatar appear instantly when upload completes!

## Upload Strategies

This example uses **WebSocket Chunked Upload**:
- ✅ Real-time progress tracking
- ✅ Handles large files efficiently (256KB chunks)
- ✅ Non-blocking uploads
- ✅ Works with LiveTemplate's reactive updates

## File Structure

```
avatar-upload/
├── main.go              # Server code with ProfileStore
├── avatar-upload.tmpl   # HTML template with upload UI
├── go.mod              # Dependencies (uses local livetemplate)
├── README.md           # This file
└── uploads/            # Created at runtime for uploaded avatars
```

## Testing Different Scenarios

### Valid Upload
- Upload a JPEG, PNG, or GIF under 5MB
- ✅ Should show progress and complete successfully

### File Too Large
- Upload an image over 5MB
- ❌ Should show validation error

### Invalid File Type
- Upload a non-image file (e.g., .txt, .pdf)
- ❌ Should show "file type not accepted" error

### Multiple Files
- Try selecting multiple images
- ℹ️ Only the first will be accepted (MaxEntries: 1)

## Code Quality

This example demonstrates:
- ✅ Clean separation of concerns (Store pattern)
- ✅ Proper error handling
- ✅ File validation and security
- ✅ Temp file cleanup
- ✅ LiveTemplate best practices

## Next Steps

Want to extend this example?

1. **Add S3 Upload**: Replace local storage with S3 presigner
2. **Multiple Avatars**: Change `MaxEntries` to allow multiple images
3. **Image Cropping**: Add client-side cropping before upload
4. **Drag & Drop**: Add drag-and-drop file selection
5. **Auto-Upload**: Set `AutoUpload: true` for instant uploads

## Learn More

- [Upload Documentation](../../livetemplate/.worktrees/feature-uploads/docs/uploads.md)
- [LiveTemplate Documentation](https://github.com/livetemplate/livetemplate)
- [Other Examples](../)

## Troubleshooting

**Upload not working?**
- Check browser console for errors
- Ensure WebSocket connection is established (look for green indicator)
- Verify file meets validation criteria (type, size)

**Progress not updating?**
- Make sure you're using WebSocket (not HTTP fallback)
- Check that ChunkSize is set appropriately
- Verify client library is loaded

**Files not saving?**
- Check that `uploads/` directory exists (created automatically)
- Verify file permissions on the uploads directory
- Check server logs for errors

---

Built with ❤️ using [LiveTemplate v0.3.0](https://github.com/livetemplate/livetemplate)
