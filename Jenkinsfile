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
        stage('Build') {
            sh "make container-build"
        }
        stage('Push [local registry]') {
            sh "make container-push-local"
        }
        stage('Tests [local registry]') {
            sh '''
                export NOCOLORS=true
                make test-e2e-k8s-local-image-container
            '''
        }
        stage('Push [hub.docker.com]') {
            withCredentials([usernamePassword(
                credentialsId: 'docker-hub-credentials',
                passwordVariable: 'DOCKER_PASS',
                usernameVariable: 'DOCKER_USER'
            )]) {
                sh '''
                    docker login -u ${DOCKER_USER} -p ${DOCKER_PASS};
                    make container-push-remote
                '''
            }
        }
        stage('Tests [k8s hub.docker.com]') {
            sh '''
                export NOCOLORS=true
                make test-e2e-k8s-remote-image-container
            '''
        }
    }
}
