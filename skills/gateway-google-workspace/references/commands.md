# Google Workspace Commands

Run commands from the `gateway-google-workspace` skill directory.

## Gmail

```bash
python3 scripts/google_workspace.py gmail-profile
python3 scripts/google_workspace.py gmail-list --query "from:alice@example.com newer_than:7d" --max-results 20
python3 scripts/google_workspace.py gmail-get --message-id MESSAGE_ID
python3 scripts/google_workspace.py gmail-send --to user@example.com --subject "Subject" --body "Plain text body"
python3 scripts/google_workspace.py gmail-draft --to user@example.com --subject "Subject" --body-file ./body.txt
```

`gmail-get` requests the full message and returns Google's payload structure. Decode individual body parts only when needed.

## Drive

```bash
python3 scripts/google_workspace.py drive-list --query "name contains 'invoice'" --page-size 50
python3 scripts/google_workspace.py drive-get --file-id FILE_ID
python3 scripts/google_workspace.py drive-create-folder --name "Project"
python3 scripts/google_workspace.py drive-create-text --name notes.txt --content "hello"
python3 scripts/google_workspace.py drive-upload --path ./report.pdf
python3 scripts/google_workspace.py drive-share-link --file-id FILE_ID
python3 scripts/google_workspace.py drive-download --file-id FILE_ID --output ./downloaded.bin
python3 scripts/google_workspace.py drive-delete --file-id FILE_ID
```

Folder creation, text creation, and upload automatically grant `anyone` + `reader`, so anyone who knows the returned `webViewLink` can view it. Before returning a link obtained from `drive-list` or `drive-get`, use `drive-share-link` to apply and verify the same permission. If Workspace policy rejects public sharing, report that the link remains restricted. Never grant `writer` by default.

`drive-download` supports stored files; export Google Docs/Sheets formats through a future export command or their native web links.
