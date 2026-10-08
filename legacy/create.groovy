import static common.*

// positional input parameters
branch = args[0]
offline = args[1]
credentials = args[2]
configurationPath = args[3]

// JIRA REST version https://docs.atlassian.com/software/jira/docs/api/REST/7.6.1/

// check manifest availability
manifest = readManifest(configurationPath)
if(!manifest) {
     err "Manifest file '${configurationPath}/manifest.json' not found"
     System.exit(1)
}

// check branch configuration availability
configuration = readBranchConfiguration(configurationPath)
if(!configuration) {
     err "Configuration file '${configurationPath}/branch.json' not found"
     System.exit(1)
}

(branchPath, issueKey) = getBranchPathAndIssueKey(manifest, configuration, branch)

userAcknowledgement = true

// check Jira
if(!offline) {

    // fetch issue details for JIRA
    jiraIssue = fetchIssue(manifest, configuration, issueKey, 'issuetype,status,assignee', credentials)

    // check branch type/issue type branching rules
    def mappedBranchPath = configuration.bitbucket.branch.mapping[jiraIssue.fields.issuetype.name]
    if(!mappedBranchPath) {
        err "No branch mapping defined for issue type '${jiraIssue.fields.issuetype.name}'"
        System.exit(1)
    }

    if(mappedBranchPath != branchPath) {
        err "Issue type '${jiraIssue.fields.issuetype.name}' is not allowed on '${branchPath}' branch"
        log "Hint: use '${mappedBranchPath}' branch or check mapping rules"
        System.exit(1)
    }

    // check issue details (more strict handling is possible)
    def assigneeName = "not yet assigned"
    def assigneeDiplayName = "nobody"
    if(jiraIssue.fields.assignee) {
        assigneeName = jiraIssue.fields.assignee.name
        assigneeDiplayName = jiraIssue.fields.assignee.displayName
    }

    log "Issue '${issueKey}' is assigned to '${assigneeDiplayName} (${assigneeName})' in status '${jiraIssue.fields.status.name}'"
    userAcknowledgement = getUserAcknowledgement()
} else {
    log "Offline mode selected (JIRA login skipped...)"
}

if(userAcknowledgement) {
    // checkout branch finally
    log "Creating branch '${branchPath}/${issueKey}'"

    (exitValue, output) = run("git checkout -b ${branchPath}/${issueKey}")
    if(!exitValue) {
        log "Hint: Use 'git branch-publish' to publish a new branch"
    }
    System.exit(exitValue)
} else {
    System.exit(0)
}

def getUserAcknowledgement() {
    // read user acknowledgement form the console
    def result
    for( def count: 1..5 ) {
        def input = readLine("%s: ", "Are you sure [y/n]?")
        switch(input) {
             case ~/^y|Y$/:
                result = true
                break
             case ~/^n|N$/:
                result = false
                break
             default:
                log "Sorry I don't understand, please try again"
                break
        }
        if(result != null) {
            break
        }
        if(count == 5) {
            log "Aborting after 5 tries"
            result = false
            break
        }

    }
    return result
}
