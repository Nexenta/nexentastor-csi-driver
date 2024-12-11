pipeline {
    parameters {
        string(
            name: 'TEST_K8S_IP',
            defaultValue: '10.3.132.116',
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
    environment {
        TESTRAIL_URL = 'https://testrail.nexenta.com/testrail'
        TESTRAIL = credentials('solutions-napalm')
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
                anyOf {
                    branch 'master'
                    branch pattern: '\\d\\.\\d\\.\\d', comparator: 'REGEXP'
                }
            }
            steps {
                sh "TEST_K8S_IP=${params.TEST_K8S_IP} TESTRAIL_URL=${TESTRAIL_URL} TESTRAIL_USR=${TESTRAIL_USR} TESTRAIL_PSWD=${TESTRAIL_PSW} make test-e2e-k8s-local-image-container"
            }
        }
        stage('Push [hub.docker.com]') {
            when {
                anyOf {
                    branch 'master'
                    branch pattern: '\\d\\.\\d\\.\\d', comparator: 'REGEXP'
                }
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
                anyOf {
                    branch 'master'
                    branch pattern: '\\d\\.\\d\\.\\d', comparator: 'REGEXP'
                }
            }
            steps {
                sh "TEST_K8S_IP=${params.TEST_K8S_IP} TESTRAIL_URL=${TESTRAIL_URL} TESTRAIL_USR=${TESTRAIL_USR} TESTRAIL_PSWD=${TESTRAIL_PSW} make test-e2e-k8s-remote-image-container"
            }
        }
    }
    post {
        success {
            slackSend color: "#317a3b", message: "${env.JOB_NAME} ${env.BUILD_NUMBER} (<${env.BUILD_URL}|Open>) - build status is success"
        }
		failure {
            slackSend color: "#e02514", message: "${env.JOB_NAME} ${env.BUILD_NUMBER} (<${env.BUILD_URL}|Open>) - build status is failed"
        }
		aborted {
            slackSend color: "#7a7a7a", message: "${env.JOB_NAME} ${env.BUILD_NUMBER} (<${env.BUILD_URL}|Open>) - build status is aborted"
		}
    }
}
