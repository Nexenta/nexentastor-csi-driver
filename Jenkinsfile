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
            steps {
                sh 'make test-e2e-ns-container'
            }
        }
        stage('Tests [csi-sanity]') {
            steps {
                sh 'make test-csi-sanity-container'
            }
        }
        stage('Push [local registry]') {
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
            environment {
                DOCKER = credentials('docker-hub-credentials')
            }
            steps {
                sh '''
                    docker login -u ${DOCKER_USR} -p ${DOCKER_PSW};
                    make container-push-remote;
                '''
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
