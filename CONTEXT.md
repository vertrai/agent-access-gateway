# Agent Access Gateway Domain

## Language

**Gateway User**: An identity that groups one or more Access Keys. A Gateway User does not directly own runtime resources.

_Avoid_: treating a Gateway User as the resource allocation boundary.

**Access Key**: A credential issued to a Gateway User and the allocation identity for gateway resources. Each Access Key owns at most one Browser and at most one Google Account.

_Avoid_: platform token, user token.

**Browser**: A persistent browser allocation, including its provider session and profile, owned by one Access Key.

**Google Account**: A managed Google Workspace account allocated from the account pool and owned by one Access Key after assignment.

**Google Account Pool**: The set of managed Google Accounts that are available for assignment or already assigned to Access Keys.
