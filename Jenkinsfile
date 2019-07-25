pipeline {
    parameters {
        string(
            name: 'TEST_K8S_IP',
            defaultValue: '10.3.199.174',
            description: 'K8s setup IP address to test on',
            trim: true
        )
        //TODO add NS parameters and generate configs for e2e tests
    }
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
                sh "TEST_K8S_IP=${params.TEST_K8S_IP} make print-variables"
                sh 'make container-build'
            }
        }
        stage('Tests [unit]') {
            steps {
                sh 'make test-unit-container'
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
                sh "TEST_K8S_IP=${params.TEST_K8S_IP} make test-e2e-k8s-local-image-container"
            }
        }
        stage('Push [hub.docker.com]') {
            when {
                branch 'master'
            }
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
                sh "TEST_K8S_IP=${params.TEST_K8S_IP} make test-e2e-k8s-remote-image-container"
            }
        }
    }
}
