node('solutions-126') {
    docker.withServer('unix:///var/run/docker.sock') {
        stage('Git clone') {
            git url: 'https://github.com/Nexenta/nexentastor-csi-driver.git', branch: 'master'
        }
        stage('Tests [unit]') {
            sh "make test-unit-container"
        }
        stage('Tests [e2e-ns]') {
            sh '''
                export NOCOLORS=true
                make test-e2e-ns-container
            '''
        }
        stage('Tests [csi-sanity]') {
            sh "make test-csi-sanity-container"
        }
    }
}
