pipeline {
    options {
        disableConcurrentBuilds()
    }
    agent {
        node {
            label 'solutions-126'
        }
    }
    stages {
        stage('Build') {
            steps {
                sh 'make container-build'
            }
        }
        stage('Tests [unit]') {
            steps {
                sh 'make test-unit-container'
            }
        }
        stage('Tests [e2e-ns]') {
            when {
                branch 'master'
            }
            steps {
                sh 'make test-e2e-ns-container'
            }
        }
        stage('Tests [csi-sanity]') {
            when {
                branch 'master'
            }
            steps {
                sh 'make test-csi-sanity-container'
            }
        }
        stage('Push [local registry]') {
            when {
                branch 'master'
            }
            steps {
                sh 'make container-push-local'
            }
        }
        stage('Tests [local registry]') {
            when {
                branch 'master'
            }
            steps {
                sh 'make test-e2e-k8s-local-image-container'
            }
        }
        stage('Push [hub.docker.com]') {
            when {
                branch 'master'
            }
            steps {
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
        }
        stage('Tests [k8s hub.docker.com]') {
            when {
                branch 'master'
            }
            steps {
                sh 'make test-e2e-k8s-remote-image-container'
            }
        }
    }
}
