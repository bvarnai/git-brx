
import groovyx.net.http.HttpBuilder
import static groovyx.net.http.HttpBuilder.configure
import static groovyx.net.http.ContentTypes.JSON
import static common.*
import groovy.json.JsonBuilder
import groovy.json.JsonOutput

// REST API information @ https://docs.atlassian.com/bitbucket-server/rest/6.1.3/bitbucket-rest.html

// positional input parameters
branch = args[0]
credentials = args[1]
configurationPath = args[2]
targetBranch = args[3]

// check manifest availability
manifest = readManifest(configurationPath)
if(!manifest) {
     err "Manifest file '${configurationPath}/manifest.json' not found"
     System.exit(1)
}

// check  branch configuration availability
configuration = readBranchConfiguration(configurationPath)
if(!configuration) {
     err "Configuration file '${configurationPath}/branch.json' not found"
     System.exit(1)
}

(branchPath, issueKey) = getBranchPathAndIssueKey(manifest, configuration, branch)

// get the list of reivewers based on the issue->components (note there might be multiple entries)
(reviewers, issue) = getReviewers()
allReviewers = reviewers.split(',')
theSelectedReviewer = allReviewers[new Random().nextInt(allReviewers.size())]

if(branchPath != 'issue' && branchPath != 'feature' && branchPath != 'epic')  {
    err "Unkown branch type ${branchPath}"
    System.exit(1)
}

trimmedIssueSummary = issue.fields.summary.trim()
reviewTemplate = new File(configurationPath, 'branch-review.template')?.text ?: 'No review template defined, please edit description manually'
reviewDescription = 
"""
${reviewTemplate}

# Merge instructions

Squash commit (preferred)
 - Title: __${issueKey} ${trimmedIssueSummary}__
 - Merge strategy: __Squash, fast-forward only (default)__
 - Delete source branch after merging: __yes__

Merge commit
 - Title: __Merge ${branch} to ${targetBranch}__
 - Merge strategy: __Merge commit__
 - Delete source branch after merging: __yes__
"""

// assemble pull-request json request
JsonBuilder reviewRequest = new JsonBuilder()
reviewRequest {
    title  "${branch}"
    description "${reviewDescription}"
    state 'OPEN'
    open true
    closed false
    fromRef {
        id "refs/heads/${branch}"
        repository {
            slug "${manifest.bitbucket.repokey}"
            name "${manifest.bitbucket.repokey}"
            project {
                key "${manifest.bitbucket.projectkey}"
            }
        }
    }
    toRef {
        id "refs/heads/${targetBranch}"
        repository {
            slug "${manifest.bitbucket.repokey}"
            name "${manifest.bitbucket.repokey}"
            project {
                key "${manifest.bitbucket.projectkey}"
            }
        }
    }
    locked false
    reviewers ( [ [ user: [ name: "${theSelectedReviewer}" ] ] ] )
}

// create pull-request in Bitbucket
_review = createReview(reviewRequest)

def getReviewers() {
    def issue = fetchIssue(manifest, configuration, issueKey, 'components,summary', credentials)
    def reviewers
    if(issue.fields.components) {
        reviewers = configuration.bitbucket.review.mapping[issue.fields.components[0].name]
        if(reviewers == null) {
            log "No review mapping defined for component type '${issue.fields.components[0].name}', fallback to 'default'"
        }
    } else {
        log "No component type is specified, fallback to 'default'"
    }
    return [reviewers ?: configuration.bitbucket.review.mapping.default, issue]
}

def createReview(reviewRequest) {
    String encodedAuthString = "Basic ${credentials}"
    def http = configure {
        request.uri = manifest.bitbucket.uri
        request.contentType = JSON[0]
        request.headers['Authorization'] = encodedAuthString

        // use proxy if set
        if(System.properties['socksProxyHost']) {
            execution.proxy(System.properties['socksProxyHost'], System.properties['socksProxyPort'] as Integer, java.net.Proxy.Type.SOCKS, false)
        }
    }.post {
        request.uri.path = "/rest/api/1.0/projects/${manifest.bitbucket.projectkey}/repos/${manifest.bitbucket.repokey}/pull-requests"
        request.body = reviewRequest.toString()
        response.failure { fs, json ->
            switch(fs.getStatusCode()) {
                case [400, 401, 403, 404, 409]:
                    // For 401, 403, 404 and 409 HTTP codes, the response will contain one or more descriptive error messages
                    json.errors.each {
                        err "${it.message}"
                    }
                    break;
                default:
                    err "Request failure (${fs.getStatusCode()})"
                break;
            }
        }
        response.success{ fs, json ->
             log "Created pull-request ${json.links.self.href}"
             return json
        }
    }
}
