import groovy.json.JsonSlurper
import groovyx.net.http.HttpBuilder
import static groovyx.net.http.HttpBuilder.configure
import static groovyx.net.http.ContentTypes.JSON
import org.fusesource.jansi.AnsiConsole

class common {

    static final STYLE_RED = "${(char)27}[31;49" + "m"
    static final STYLE_DEFAULT= "${(char)27}[39;49" + "m"

    static run(command, bash=false) {
        if (bash) {
            def env = System.getenv()
            // run command in bash
            command = "${env['BASH_HOME']}\\bash -c \"${command}\""
            println "$command"
        }
        // run process
        def process = "${command}".execute()
        def outputStream = new StringBuffer();
        process.waitForProcessOutput(outputStream, System.err)
        if(!process.exitValue()) {
            println outputStream
        }

        return [process.exitValue(), outputStream]
    }

    // mimic bash logging
    static log(message) {
        println "[branch] " + message
    }

    static err(message) {
        AnsiConsole.systemInstall()
        println "[branch] " + STYLE_RED + "! " + message + STYLE_DEFAULT
        AnsiConsole.systemUninstall()
    }

    static readLine(format, args) {
        def line
        if (System.console() != null) {
            line = System.console().readLine(format, args)
        } else {
            print String.format(format, args)
            BufferedReader reader = new BufferedReader(new InputStreamReader(System.in))
            line = reader.readLine()
        }
        return line
    }

    static getBranchPathAndIssueKey(manifest, configuration, branch) {
        // check generic branch pattern
        assert configuration.bitbucket.branch.mapping instanceof Map
        assert configuration.bitbucket.branch.template

        // assemble the regexp template
        //def binding = [allowedpaths: configuration.bitbucket.branch.mapping.values().toSet().join('|'), projectkey: configuration.jira.projectkey]
        def binding = [projectkey: manifest.jira.projectkey]
        def engine = new groovy.text.SimpleTemplateEngine()
        def template = engine.createTemplate(configuration.bitbucket.branch.template).make(binding)
        def pattern = template.toString()

        def matcher = branch =~ /${pattern}/
        if(!matcher.matches()) {
            err "Branch name '${branch}' doesn't match pattern ${pattern}"
            System.exit(1)
        }

        // branchPath, issueKey
        return [matcher.group(1), matcher.group(2)]
    }

    static readBranchConfiguration(configurationPath) {
        def branchConfigurationFile = new File(configurationPath, "branch.json")
        if (branchConfigurationFile.exists() && branchConfigurationFile.isFile()) {
            def jsonSlurper = new JsonSlurper()
            def branchConfigurationJson = jsonSlurper.parse(branchConfigurationFile)
            return branchConfigurationJson
        }
    }

    static readManifest(configurationPath) {
        def manifestFile = new File(configurationPath, "manifest.json")
        if (manifestFile.exists() && manifestFile.isFile()) {
            // read repo-wide configuration
            def jsonSlurper = new JsonSlurper()
            def manifestJson = jsonSlurper.parse(manifestFile)
            return manifestJson
        }
    }

    static fetchIssue(manifest, configuration, issueKey, fields, credentials) {
        String encodedAuthString = "Basic ${credentials}"
        def http = configure {
            request.uri = manifest.jira.uri
            request.contentType = JSON[0]
            request.headers['Authorization'] = encodedAuthString

            // use proxy if set
            if(System.properties['socksProxyHost']) {
                execution.proxy(System.properties['socksProxyHost'], System.properties['socksProxyPort'] as Integer, java.net.Proxy.Type.SOCKS, false)
            }
        }.get {
            request.uri.path = "/rest/api/2/issue/${issueKey}"
            // pull all necessary fields
            request.uri.query = [fields: fields]
            response.failure { fs, json ->
                // we not always get a nice response
                if(fs.getContentType().startsWith(JSON[0])) {
                    json.errorMessages.each {
                        err "${it}"
                    }
                } else {
                    // generic stuff
                    err "Authorization failure (${fs.getStatusCode()})"
                    log "Hint: CAPTHA protection might be active, try to login manually"
                }
                System.exit(1)
            }
            response.success{ fs, json ->
                  return json
            }
        }
    }
}
