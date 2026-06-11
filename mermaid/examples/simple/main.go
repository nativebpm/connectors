package main

import (
	"fmt"
	"os"

	"github.com/nativebpm/bpmn"
	"github.com/nativebpm/connectors/mermaid"
)

func main() {
	fmt.Println("==================================================")
	fmt.Println("⚡ NATIVEBPM BPMN-TO-MERMAID TRANSLATOR DEMO ⚡")
	fmt.Println("==================================================")

	// 1. Build a BPMN workflow using the fluent builder API (Workflow as Code)
	builder := bpmn.NewBuilder("customer_onboarding", "Customer Onboarding Flow").
		StartEvent("start").Next("verify_email").
		ServiceTask("verify_email", "Verify Email Address", "email_verification").Next("check_credit").
		ServiceTask("check_credit", "Check Credit Score", "credit_scoring").Next("gateway_approve").
		ExclusiveGateway("gateway_approve", "Is Approved?").
			Condition("welcome_user", bpmn.Var("approved").Eq(true)).
			Default("reject_user").
			Builder().
		ServiceTask("welcome_user", "Send Welcome Packet", "welcome_email").Next("success_end").
		ServiceTask("reject_user", "Send Rejection Email", "rejection_email").Next("failed_end").
		EndEvent("success_end", "User Onboarded").
		EndEvent("failed_end", "Registration Rejected")

	// 2. Export BPMN builder to raw XML data
	xmlData, err := builder.BuildXML()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error compiling BPMN XML: %v\n", err)
		os.Exit(1)
	}

	// 3. Parse XML using BPMN parser to generate execution graph
	processes, err := bpmn.ParseBPMN(xmlData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing BPMN XML: %v\n", err)
		os.Exit(1)
	}
	pp := processes["customer_onboarding"]

	// 4. Translate parsed execution graph to Mermaid flowchart
	mermaidMD, err := mermaid.Generate(pp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating Mermaid: %v\n", err)
		os.Exit(1)
	}

	// 5. Print out the resulting Mermaid markdown
	fmt.Println("Successfully generated Mermaid flowchart:")
	fmt.Println(mermaidMD)
	fmt.Println("==================================================")
}
